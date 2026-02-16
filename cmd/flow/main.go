// Command-line interface for the Flow framework.
//
// This file implements a small, user-facing CLI using cobra. It provides
// a `serve` command to run an App and a `version` command. The CLI is
// intentionally minimal but fully functional so it can be extended with
// generators and other developer tools later.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	routerpkg "github.com/undiegomejia/flow/internal/router"
	flowpkg "github.com/undiegomejia/flow/pkg/flow"
	"github.com/undiegomejia/flow/pkg/observability"
	"github.com/undiegomejia/flow/pkg/plugins"

	gen "github.com/undiegomejia/flow/internal/generator"
	// include the sample plugin so the CLI binary built during tests
	// includes an example generator registered via init().
	_ "github.com/undiegomejia/flow/pkg/plugins/sample"
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

	rootCmd := &cobra.Command{Use: "app"}
	// core serve command (starts the app)
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(generateCmd)
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate code artifacts",
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
			fmt.Fprintln(out, "no generator plugins registered")
			return nil
		}
		// output flags: format and quiet
		format, _ := cmd.Flags().GetString("format")
		quiet, _ := cmd.Flags().GetBool("quiet")

		// quiet mode: only print names one per line
		if quiet {
			for _, name := range names {
				fmt.Fprintln(out, name)
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
		fmt.Fprintln(tw, "NAME\tVERSION\tHELP")
		for _, name := range names {
			g := gen.GetRegisteredGenerator(name)
			if g == nil {
				fmt.Fprintf(tw, "%s\t-\t-\n", name)
				continue
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\n", g.Name(), g.Version(), trim(g.Help(), maxHelp))
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
}
