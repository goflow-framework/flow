// Command-line interface for the Flow framework.
//
// This file implements a small, user-facing CLI using cobra. It provides
// a `serve` command to run an App, a `new` command to scaffold new projects,
// a `db` group with `migrate` / `rollback` / `status` subcommands wired to
// internal/migrations, and generator subcommands under `generate`.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"text/tabwriter"
	"time"

	gen "github.com/goflow-framework/flow/internal/generator"
	migrations "github.com/goflow-framework/flow/internal/migrations"
	routerpkg "github.com/goflow-framework/flow/internal/router"
	flowpkg "github.com/goflow-framework/flow/pkg/flow"
	"github.com/goflow-framework/flow/pkg/observability"
	"github.com/goflow-framework/flow/pkg/plugins"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"

	// register the pure-Go SQLite driver used by db migrate in development
	_ "modernc.org/sqlite"
	// include the sample plugin so the CLI binary built during tests
	// includes an example generator registered via init().
	_ "github.com/goflow-framework/flow/pkg/plugins/sample"
)

var (
	generateTarget string
	serveAddr      string
	metricsAddr    string
	traceStdout    bool
	serviceName    string
	otlpEndpoint   string
	otlpInsecure   bool
	otlpHeaders    string

	// db command flags
	dbMigrationsDir string
	dbDSN           string
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancel()
	}()

	rootCmd := &cobra.Command{
		Use:   "flow",
		Short: "Flow – opinionated Go web framework CLI",
		Long: `flow is the developer CLI for the Flow web framework.

Use it to scaffold new projects, run the development server, manage
database migrations, and generate code artifacts.`,
	}
	// core commands
	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(dbCmd)
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate code artifacts",
}

// ---------------------------------------------------------------------------
// flow new
// ---------------------------------------------------------------------------

var newCmd = &cobra.Command{
	Use:   "new <app-name>",
	Short: "Scaffold a new Flow application",
	Long: `Scaffold a new Flow application directory with a minimal, opinionated
layout ready for development.

The generated structure includes:
  <app-name>/
    cmd/<app-name>/main.go          – application entry-point
    cmd/<app-name>/main_test.go     – smoke test for the entry-point package
    internal/<app-name>_test.go     – placeholder integration test
    db/migrations/                  – SQL migration files
    go.mod                          – module definition
    .env.example                    – documented environment variables
    Makefile                        – common developer tasks (includes 'make test')`,
	Args: cobra.ExactArgs(1),
	RunE: runNew,
}

func runNew(cmd *cobra.Command, args []string) error {
	name := args[0]
	root := name

	// Refuse to overwrite an existing directory
	if _, err := os.Stat(root); err == nil {
		return fmt.Errorf("directory %q already exists", root)
	}

	dirs := []string{
		filepath.Join(root, "cmd", name),
		filepath.Join(root, "internal"),
		filepath.Join(root, "db", "migrations"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", d, err)
		}
	}

	files := map[string]string{
		filepath.Join(root, "go.mod"): fmt.Sprintf("module %s\n\ngo 1.24\n\nrequire github.com/goflow-framework/flow v0.0.0\n", name),

		filepath.Join(root, "cmd", name, "main.go"): fmt.Sprintf(`package main

import (
	"log"

	"github.com/goflow-framework/flow/pkg/flow"
)

func main() {
	app := flow.New(%q,
		flow.WithDefaultMiddleware(),
	)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
`, name),

		filepath.Join(root, "cmd", name, "main_test.go"): fmt.Sprintf(`package main

import (
	"testing"
)

// TestSmoke verifies the application entry-point package compiles and the
// test harness is wired up correctly. Replace this with meaningful tests
// as your application grows.
func TestSmoke(t *testing.T) {
	t.Logf("%s: smoke test passed – add real tests here")
}
`, name),

		filepath.Join(root, "internal", name+"_test.go"): fmt.Sprintf(`package internal_test

import (
	"testing"
)

// TestIntegration is a placeholder for %s integration tests.
// Wire up your app, make HTTP requests against a test server, and assert
// the responses here.
func TestIntegration(t *testing.T) {
	t.Skip("replace with real integration tests")
}
`, name),

		filepath.Join(root, ".env.example"): `# Runtime environment: development | test | production
FLOW_ENV=development

# HTTP listen address
FLOW_ADDR=:3000

# Primary database DSN (postgres or sqlite)
DATABASE_URL=sqlite://db/development.db

# Session signing secret – set to a random 32+ char string in production
# FLOW_SECRET_KEY_BASE=

# Logging verbosity: debug | info | warn | error
FLOW_LOG_LEVEL=info
`,

		filepath.Join(root, "Makefile"): fmt.Sprintf(`# Common developer tasks for %s

.PHONY: run build test migrate rollback

run:
	go run ./cmd/%s

build:
	go build -o bin/%s ./cmd/%s

test:
	go test ./... -v

migrate:
	flow db migrate --dir db/migrations

rollback:
	flow db rollback --dir db/migrations
`, name, name, name, name),
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	fmt.Printf("✓ Created new Flow application: %s\n\n", name)
	fmt.Printf("  cd %s\n", name)
	fmt.Printf("  cp .env.example .env\n")
	fmt.Printf("  go mod tidy\n")
	fmt.Printf("  make run\n\n")
	return nil
}

// ---------------------------------------------------------------------------
// flow db
// ---------------------------------------------------------------------------

// openDB opens a *sql.DB from dsn, falling back to DATABASE_URL env var.
// It infers the driver from the DSN prefix:
//   - "postgres://" or "postgresql://" → "pgx" (requires pgx driver)
//   - everything else (including "sqlite://") → "sqlite" (modernc pure-Go)
func openDB(dsn string) (*sql.DB, error) {
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		return nil, fmt.Errorf("no database DSN: set --dsn or DATABASE_URL")
	}
	driver := "sqlite"
	rawDSN := dsn
	switch {
	case len(dsn) >= 11 && dsn[:11] == "postgresql://":
		driver = "pgx"
	case len(dsn) >= 11 && dsn[:11] == "postgres://":
		driver = "pgx"
	case len(dsn) >= 9 && dsn[:9] == "sqlite://":
		// strip the scheme so modernc/sqlite receives a plain file path
		rawDSN = dsn[9:]
	}
	return sql.Open(driver, rawDSN)
}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database management commands",
}

var dbMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Apply all pending SQL migrations",
	Long: `Apply all pending .up.sql migrations found in the migrations directory.

Migrations are tracked in a flow_migrations table so each file is only
applied once. Files are applied in ascending lexicographic (timestamp) order.

The target database is resolved from --dsn or the DATABASE_URL environment
variable.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openDB(dbDSN)
		if err != nil {
			return err
		}
		defer db.Close()

		runner := &migrations.MigrationRunner{}
		pending, err := runner.PendingMigrations(dbMigrationsDir, db)
		if err != nil {
			return fmt.Errorf("db migrate: %w", err)
		}
		if len(pending) == 0 {
			fmt.Println("Nothing to migrate – all migrations are already applied.")
			return nil
		}
		fmt.Printf("Applying %d migration(s)...\n", len(pending))
		if err := runner.ApplyAll(dbMigrationsDir, db); err != nil {
			return fmt.Errorf("db migrate: %w", err)
		}
		for _, name := range pending {
			fmt.Printf("  ✓ %s\n", name)
		}
		fmt.Println("Done.")
		return nil
	},
}

var dbRollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Roll back the last applied migration",
	Long: `Execute the .down.sql file for the most recently applied migration and
remove it from the tracking table.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openDB(dbDSN)
		if err != nil {
			return err
		}
		defer db.Close()

		runner := &migrations.MigrationRunner{}
		if err := runner.RollbackLast(dbMigrationsDir, db); err != nil {
			return fmt.Errorf("db rollback: %w", err)
		}
		fmt.Println("✓ Rolled back last migration.")
		return nil
	},
}

var dbStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show applied and pending migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, err := openDB(dbDSN)
		if err != nil {
			return err
		}
		defer db.Close()

		runner := &migrations.MigrationRunner{}

		applied, err := runner.AppliedMigrations(db)
		if err != nil {
			return fmt.Errorf("db status: %w", err)
		}
		pending, err := runner.PendingMigrations(dbMigrationsDir, db)
		if err != nil {
			return fmt.Errorf("db status: %w", err)
		}

		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 8, 2, ' ', 0)
		if _, err := fmt.Fprintln(tw, "STATUS\tMIGRATION"); err != nil {
			return err
		}
		for _, name := range applied {
			if _, err := fmt.Fprintf(tw, "applied\t%s\n", name); err != nil {
				return err
			}
		}
		for _, name := range pending {
			if _, err := fmt.Fprintf(tw, "pending\t%s\n", name); err != nil {
				return err
			}
		}
		if len(applied) == 0 && len(pending) == 0 {
			if _, err := fmt.Fprintln(tw, "(no migrations found)"); err != nil {
				return err
			}
		}
		return tw.Flush()
	},
}

var genControllerCmd = &cobra.Command{
	Use:   "controller [name]",
	Short: "Generate a new controller",
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
	Short: "Generate a new model",
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
		noI18n, _ := cmd.Flags().GetBool("no-i18n")
		opts := gen.GenOptions{Force: force, SkipMigrations: skipMigs, NoViews: noViews}
		opts.NoI18n = noI18n
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

var genAdminCmd = &cobra.Command{
	Use:   "admin [name] [fields...]",
	Short: "Generate admin UI for a resource",
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
		noI18n, _ := cmd.Flags().GetBool("no-i18n")
		opts := gen.GenOptions{Force: force, NoI18n: noI18n}
		created, err := gen.GenerateAdminWithOptions(root, name, opts, fields...)
		if err != nil {
			return err
		}
		for _, c := range created {
			fmt.Println("created", c)
		}
		return nil
	},
}

var genAuthCmd = &cobra.Command{
	Use:   "auth",
	Short: "Generate auth scaffold (User model, controller, middleware)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		root := generateTarget
		if root == "" {
			var err error
			root, err = os.Getwd()
			if err != nil {
				return err
			}
		}
		force, _ := cmd.Flags().GetBool("force")
		noI18n, _ := cmd.Flags().GetBool("no-i18n")
		opts := gen.GenOptions{Force: force, NoI18n: noI18n}
		created, err := gen.GenerateAuthWithOptions(root, opts)
		if err != nil {
			return err
		}
		for _, c := range created {
			fmt.Println("created", c)
		}
		return nil
	},
}

var genPluginCmd = &cobra.Command{
	Use:   "plugin [name] [args...]",
	Short: "Run a registered generator plugin",
	Args:  cobra.MinimumNArgs(1),
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
		g := gen.GetRegisteredGenerator(name)
		if g == nil {
			return fmt.Errorf("generator plugin not found: %s", name)
		}
		created, err := g.Generate(root, args[1:])
		if err != nil {
			return err
		}
		for _, c := range created {
			fmt.Println("created", c)
		}
		return nil
	},
}

var genListCmd = &cobra.Command{
	Use:     "plugins",
	Aliases: []string{"list"},
	Short:   "List registered generator plugins",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		names := gen.ListRegisteredGenerators()
		out := cmd.OutOrStdout()
		if len(names) == 0 {
			if _, err := fmt.Fprintln(out, "no generator plugins registered"); err != nil {
				return err
			}
			return nil
		}
		// output flags: format and quiet
		format, _ := cmd.Flags().GetString("format")
		quiet, _ := cmd.Flags().GetBool("quiet")

		// quiet mode: only print names one per line
		if quiet {
			for _, name := range names {
				if _, err := fmt.Fprintln(out, name); err != nil {
					return err
				}
			}
			return nil
		}

		if format == "json" {
			type info struct {
				Name    string `json:"name"`
				Version string `json:"version"`
				Help    string `json:"help"`
			}
			var arr []info
			for _, name := range names {
				g := gen.GetRegisteredGenerator(name)
				if g == nil {
					arr = append(arr, info{Name: name})
					continue
				}
				arr = append(arr, info{Name: g.Name(), Version: g.Version(), Help: g.Help()})
			}
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			return enc.Encode(arr)
		}

		// default table output with trimmed help column
		const maxHelp = 100
		trim := func(s string, n int) string {
			if len(s) <= n {
				return s
			}
			if n <= 3 {
				return s[:n]
			}
			return s[:n-3] + "..."
		}

		tw := tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)
		defer tw.Flush()
		if _, err := fmt.Fprintln(tw, "NAME\tVERSION\tHELP"); err != nil {
			return err
		}
		for _, name := range names {
			g := gen.GetRegisteredGenerator(name)
			if g == nil {
				if _, err := fmt.Fprintf(tw, "%s\t-\t-\n", name); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\n", g.Name(), g.Version(), trim(g.Help(), maxHelp)); err != nil {
				return err
			}
		}
		return nil
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the development server",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Simple in-process serve command. Construct the app and start it.
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
				srv := &http.Server{
					Addr:         metricsAddr,
					Handler:      mux,
					ReadTimeout:  5 * time.Second,
					WriteTimeout: 10 * time.Second,
					IdleTimeout:  30 * time.Second,
				}
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

		// apply plugins before serving
		if err := plugins.ApplyAll(app); err != nil {
			return err
		}

		// start and block until signal
		if err := app.Start(); err != nil {
			return err
		}

		// Wait for shutdown signal
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig

		// First shut down the app (stop accepting requests, flush
		// internal resources). After the app is shut down, run any
		// plugin shutdown hooks so plugins can clean up their own
		// resources.
		if err := app.Shutdown(context.Background()); err != nil {
			return err
		}

		if err := plugins.ShutdownAll(context.Background()); err != nil {
			return err
		}

		return nil
	},
}

func init() {
	generateCmd.AddCommand(genControllerCmd)
	generateCmd.AddCommand(genModelCmd)
	generateCmd.AddCommand(genScaffoldCmd)
	generateCmd.AddCommand(genAdminCmd)
	generateCmd.AddCommand(genAuthCmd)
	generateCmd.AddCommand(genPluginCmd)
	// list plugins (alias: list)
	generateCmd.AddCommand(genListCmd)
	genListCmd.Flags().String("format", "table", "output format: table|json")
	genListCmd.Flags().Bool("quiet", false, "quiet output: print only generator names")
	genControllerCmd.Flags().Bool("force", false, "overwrite existing files")
	genModelCmd.Flags().Bool("force", false, "overwrite existing files")
	// genRoutesCmd is defined in gen_routes.go
	genScaffoldCmd.Flags().Bool("force", false, "overwrite existing files")
	genScaffoldCmd.Flags().Bool("no-i18n", false, "do not generate i18n translation files")
	genAdminCmd.Flags().Bool("force", false, "overwrite existing files")
	genAdminCmd.Flags().Bool("no-i18n", false, "do not generate i18n translation files")
	genAuthCmd.Flags().Bool("force", false, "overwrite existing files")
	genAuthCmd.Flags().Bool("no-i18n", false, "do not generate i18n translation files")
	genScaffoldCmd.Flags().Bool("skip-migrations", false, "do not create migration files")
	genScaffoldCmd.Flags().Bool("no-views", false, "do not generate view files")
	generateCmd.PersistentFlags().StringVar(&generateTarget, "target", "", "target project root (defaults to cwd)")

	serveCmd.Flags().StringVar(&serveAddr, "addr", ":3000", "listen address for the server")
	serveCmd.Flags().StringVar(&metricsAddr, "metrics-addr", "", "optional address to expose Prometheus /metrics (e\n g. :9090)")
	serveCmd.Flags().BoolVar(&traceStdout, "trace-stdout", false, "enable stdout OpenTelemetry tracer for local deb\n ugging")
	serveCmd.Flags().StringVar(&serviceName, "service-name", "flow", "service.name used by tracing exporter")
	serveCmd.Flags().StringVar(&otlpEndpoint, "otlp-endpoint", "", "OTLP gRPC endpoint (e.g. otel-collector:4317)")
	serveCmd.Flags().BoolVar(&otlpInsecure, "otlp-insecure", false, "use insecure gRPC connection for OTLP (local collector)")
	serveCmd.Flags().StringVar(&otlpHeaders, "otlp-headers", "", "comma-separated key=val headers for OTLP (e.g. api-key=foo)")

	// db subcommands
	dbCmd.AddCommand(dbMigrateCmd)
	dbCmd.AddCommand(dbRollbackCmd)
	dbCmd.AddCommand(dbStatusCmd)
	// shared persistent flags for all db subcommands
	dbCmd.PersistentFlags().StringVar(&dbMigrationsDir, "dir", "db/migrations", "directory containing SQL migration files")
	dbCmd.PersistentFlags().StringVar(&dbDSN, "dsn", "", "database DSN (overrides DATABASE_URL env var)")
}
