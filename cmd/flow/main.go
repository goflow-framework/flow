// Command-line interface for the Flow framework.
//
// This file implements a small, user-facing CLI using cobra. It provides
// a `serve` command to run an App and a `version` command. The CLI is
// intentionally minimal but fully functional so it can be extended with
// generators and other developer tools later.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"database/sql"
	routerpkg "github.com/dministrator/flow/internal/router"
	flowpkg "github.com/dministrator/flow/pkg/flow"
	"github.com/dministrator/flow/pkg/observability"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	gen "github.com/dministrator/flow/internal/generator"
	mig "github.com/dministrator/flow/internal/migrations"
)

const version = "0.1.0"

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "flow",
	Short: "Flow — an opinionated Go MVC web framework (CLI)",
	Long:  "Flow CLI: run, generate and manage Flow web applications.",
}

func init() {
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(dbCmd)
	rootCmd.AddCommand(generateCmd)
}

var serveAddr string
var metricsAddr string
var traceStdout bool
var serviceName string
var otlpEndpoint string
var otlpInsecure bool
var otlpHeaders string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the development server",
	RunE: func(cmd *cobra.Command, args []string) error {
		// check flags
		watch, _ := cmd.Flags().GetBool("watch")
		noWatch, _ := cmd.Flags().GetBool("no-watch")
		if watch && !noWatch {
			// run watcher which spawns go run ./cmd/flow serve --no-watch ...
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			// read watch paths and ignore patterns from flags
			watchPaths, _ := cmd.Flags().GetStringSlice("watch-paths")
			if len(watchPaths) == 0 {
				watchPaths = []string{"."}
			}
			ignorePatterns, _ := cmd.Flags().GetStringSlice("watch-ignore")
			extFilters, _ := cmd.Flags().GetStringSlice("watch-ext")
			// build child args: serve --no-watch --addr <addr>
			childArgs := []string{"serve", "--no-watch", "--addr", serveAddr}
			return WatchAndRun(ctx, watchPaths, ignorePatterns, extFilters, childArgs)
		}

		// Normal in-process serve (or --no-watch child)
		// Use the default middleware stack in demo CLI server to provide
		// sensible defaults (recovery, request-id, logging, metrics).
		app := flowpkg.New("flow", flowpkg.WithAddr(serveAddr), flowpkg.WithDefaultMiddleware())

		// small demo router: exposes a health endpoint and root index
		r := routerpkg.New()
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(200)
			_, _ = w.Write([]byte("Flow app running"))
		})
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(200)
			_, _ = w.Write([]byte("{\"status\":\"ok\"}"))
		})

		app.SetRouter(r)

		// wire Prometheus middleware and optional tracer
		if metricsAddr != "" {
			app.Use(observability.InstrumentHandler)
			go func() {
				mux := http.NewServeMux()
				mux.Handle("/metrics", promhttp.Handler())
				if err := http.ListenAndServe(metricsAddr, mux); err != nil {
					fmt.Fprintln(os.Stderr, "metrics server error:", err)
				}
			}()
		}

		// Tracing: prefer OTLP exporter when endpoint provided, otherwise fall back to stdout tracer.
		var tracerShutdown func(context.Context) error
		if otlpEndpoint != "" {
			headersMap := observability.ParseHeaders(otlpHeaders)
			shutdown, err := observability.SetupOTLPTracer(context.Background(), otlpEndpoint, otlpInsecure, headersMap, serviceName)
			if err != nil {
				fmt.Fprintln(os.Stderr, "failed to setup OTLP tracer:", err)
			} else {
				tracerShutdown = shutdown
				defer func() { _ = tracerShutdown(context.Background()) }()
			}
		} else if traceStdout {
			shutdown, err := observability.SetupStdoutTracer(serviceName, observability.StdoutTracerOptions{})
			if err != nil {
				fmt.Fprintln(os.Stderr, "failed to setup tracer:", err)
			} else {
				tracerShutdown = shutdown
				defer func() { _ = tracerShutdown(context.Background()) }()
			}
		}

		// start and block until signal
		if err := app.Start(); err != nil {
			return err
		}

		// Wait for shutdown signal
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig

		return app.Shutdown(context.Background())
	},
}

func init() {
	serveCmd.Flags().StringVar(&serveAddr, "addr", ":3000", "listen address for the server")
	serveCmd.Flags().StringVar(&metricsAddr, "metrics-addr", "", "optional address to expose Prometheus /metrics (eg. :9090)")
	serveCmd.Flags().BoolVar(&traceStdout, "trace-stdout", false, "enable stdout OpenTelemetry tracer for local debugging")
	serveCmd.Flags().StringVar(&serviceName, "service-name", "flow", "service.name used by tracing exporter")
	serveCmd.Flags().Bool("watch", false, "watch files and auto-restart server on changes")
	// internal flag used by watcher to avoid recursive watch
	serveCmd.Flags().Bool("no-watch", false, "(internal) do not start file watcher")
	serveCmd.Flags().StringSlice("watch-paths", []string{"."}, "paths to watch (comma-separated)")
	serveCmd.Flags().StringSlice("watch-ignore", []string{".git", "vendor", "node_modules"}, "paths or patterns to ignore (comma-separated)")
	serveCmd.Flags().StringSlice("watch-ext", []string{".go", ".tmpl", ".html", ".sql"}, "file extensions to trigger restarts (e.g. .go,.tmpl). Empty => watch all files")
	// OTLP flags (match behavior in flow-worker)
	serveCmd.Flags().StringVar(&otlpEndpoint, "otlp-endpoint", "", "OTLP gRPC endpoint (e.g. otel-collector:4317)")
	serveCmd.Flags().BoolVar(&otlpInsecure, "otlp-insecure", false, "use insecure gRPC connection for OTLP (local collector)")
	serveCmd.Flags().StringVar(&otlpHeaders, "otlp-headers", "", "comma-separated key=val headers for OTLP (e.g. api-key=foo)")
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("flow", version)
	},
}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database tasks (migrate, rollback)",
}

var dbDir string
var dbDriver string
var dbDSN string

var dbMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Apply all pending migrations in a directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dbDriver == "" || dbDSN == "" {
			return fmt.Errorf("driver and dsn flags are required to run migrations")
		}
		db, err := sql.Open(dbDriver, dbDSN)
		if err != nil {
			return err
		}
		defer db.Close()
		runner := &mig.MigrationRunner{}

		// list applied before
		appliedBefore, err := runner.AppliedMigrations(db)
		if err != nil {
			return err
		}

		pending, err := runner.PendingMigrations(dbDir, db)
		if err != nil {
			return err
		}
		if len(pending) == 0 {
			fmt.Println("No pending migrations to apply.")
			return nil
		}
		fmt.Println("Pending migrations:")
		for _, p := range pending {
			fmt.Println(" -", p)
		}

		if err := runner.ApplyAll(dbDir, db); err != nil {
			return err
		}

		// list newly applied
		appliedAfter, err := runner.AppliedMigrations(db)
		if err != nil {
			return err
		}
		// compute diff appliedAfter - appliedBefore
		beforeSet := make(map[string]struct{}, len(appliedBefore))
		for _, b := range appliedBefore {
			beforeSet[b] = struct{}{}
		}
		var newly []string
		for _, a := range appliedAfter {
			if _, ok := beforeSet[a]; !ok {
				newly = append(newly, a)
			}
		}
		if len(newly) == 0 {
			fmt.Println("No new migrations were applied.")
			return nil
		}
		fmt.Println("Applied migrations:")
		for _, n := range newly {
			fmt.Println(" -", n)
		}
		return nil
	},
}

var dbRollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Rollback the most recent migration",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dbDriver == "" || dbDSN == "" {
			return fmt.Errorf("driver and dsn flags are required to rollback migrations")
		}
		db, err := sql.Open(dbDriver, dbDSN)
		if err != nil {
			return err
		}
		defer db.Close()
		runner := &mig.MigrationRunner{}

		applied, err := runner.AppliedMigrations(db)
		if err != nil {
			return err
		}
		if len(applied) == 0 {
			fmt.Println("No applied migrations found; nothing to rollback.")
			return nil
		}
		last := applied[len(applied)-1]
		fmt.Println("Rolling back migration:", last)
		if err := runner.RollbackLast(dbDir, db); err != nil {
			return err
		}
		fmt.Println("Rolled back:", last)
		return nil
	},
}

var dbStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show applied and pending migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dbDriver == "" || dbDSN == "" {
			return fmt.Errorf("driver and dsn flags are required to check status")
		}
		db, err := sql.Open(dbDriver, dbDSN)
		if err != nil {
			return err
		}
		defer db.Close()
		runner := &mig.MigrationRunner{}
		applied, err := runner.AppliedMigrations(db)
		if err != nil {
			return err
		}
		pending, err := runner.PendingMigrations(dbDir, db)
		if err != nil {
			return err
		}
		fmt.Println("Applied migrations:")
		if len(applied) == 0 {
			fmt.Println(" (none)")
		} else {
			for _, a := range applied {
				fmt.Println(" -", a)
			}
		}
		fmt.Println("Pending migrations:")
		if len(pending) == 0 {
			fmt.Println(" (none)")
		} else {
			for _, p := range pending {
				fmt.Println(" -", p)
			}
		}
		return nil
	},
}

func init() {
	dbCmd.AddCommand(dbMigrateCmd)
	dbCmd.AddCommand(dbRollbackCmd)
	dbCmd.AddCommand(dbStatusCmd)
	dbCmd.PersistentFlags().StringVar(&dbDir, "dir", "db/migrate", "migrations directory")
	dbCmd.PersistentFlags().StringVar(&dbDriver, "driver", "", "database driver (eg. postgres, mysql)")
	dbCmd.PersistentFlags().StringVar(&dbDSN, "dsn", "", "database DSN")
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Code generators (controller, model, scaffold)",
}

var generateTarget string

var genControllerCmd = &cobra.Command{
	Use:   "controller [name]",
	Short: "Generate a controller",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		root := generateTarget
		if root == "" {
			var err error
			root, err = os.Getwd()
			if err != nil {
				return err
			}
		}
		// read flags
		force, _ := cmd.Flags().GetBool("force")
		opts := gen.GenOptions{Force: force}
		dst, err := gen.GenerateControllerWithOptions(root, name, opts)
		if err != nil {
			return err
		}
		fmt.Println("created", dst)
		return nil
	},
}

var genModelCmd = &cobra.Command{
	Use:   "model [name] [fields...]",
	Short: "Generate a model (optionally with fields, e.g. title:string published_at:datetime)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		fields := []string{}
		if len(args) > 1 {
			fields = args[1:]
		}
		root := generateTarget
		if root == "" {
			var err error
			root, err = os.Getwd()
			if err != nil {
				return err
			}
		}
		force, _ := cmd.Flags().GetBool("force")
		// model generation currently supports --force to overwrite
		opts := gen.GenOptions{Force: force}
		dst, err := gen.GenerateModelWithOptions(root, name, opts, fields...)
		if err != nil {
			return err
		}
		fmt.Println("created", dst)
		return nil
	},
}

var genScaffoldCmd = &cobra.Command{
	Use:   "scaffold [name] [fields...]",
	Short: "Generate scaffold (controller, model, views) optionally with fields",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		fields := []string{}
		if len(args) > 1 {
			fields = args[1:]
		}
		root := generateTarget
		if root == "" {
			var err error
			root, err = os.Getwd()
			if err != nil {
				return err
			}
		}
		force, _ := cmd.Flags().GetBool("force")
		skipMigs, _ := cmd.Flags().GetBool("skip-migrations")
		noViews, _ := cmd.Flags().GetBool("no-views")
		opts := gen.GenOptions{Force: force, SkipMigrations: skipMigs, NoViews: noViews}
		created, err := gen.GenerateScaffoldWithOptions(root, name, opts, fields...)
		if err != nil {
			return err
		}
		for _, c := range created {
			fmt.Println("created", c)
		}
		return nil
	},
}

func init() {
	generateCmd.AddCommand(genControllerCmd)
	generateCmd.AddCommand(genModelCmd)
	generateCmd.AddCommand(genScaffoldCmd)
	genControllerCmd.Flags().Bool("force", false, "overwrite existing files")
	genModelCmd.Flags().Bool("force", false, "overwrite existing files")
	genScaffoldCmd.Flags().Bool("force", false, "overwrite existing files")
	genScaffoldCmd.Flags().Bool("skip-migrations", false, "do not create migration files")
	genScaffoldCmd.Flags().Bool("no-views", false, "do not generate view files")
	generateCmd.PersistentFlags().StringVar(&generateTarget, "target", "", "target project root (defaults to cwd)")
}
