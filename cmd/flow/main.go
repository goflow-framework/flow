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
        "os"
        "os/signal"
        "syscall"

        "github.com/spf13/cobra"

        "database/sql"
        routerpkg "github.com/dministrator/flow/internal/router"
        flowpkg "github.com/dministrator/flow/pkg/flow"
        "github.com/dministrator/flow/pkg/plugins"
        "net/http"

        gen "github.com/dministrator/flow/internal/generator"
        rg "github.com/dministrator/flow/internal/generator/routegen"
        mig "github.com/dministrator/flow/internal/migrations"
        "path/filepath"
)

var (
        db          *sql.DB
        generateTarget string
        serveAddr string
        metricsAddr string
        traceStdout bool
        serviceName string
        otlpEndpoint string
        otlpInsecure bool
        otlpHeaders string
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
        rootCmd.AddCommand(routerpkg.RouterCmd)
        rootCmd.AddCommand(flowpkg.FlowCmd)
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
                force, _ := cmd.Flags().GetBool("force")
                return gen.GenerateController(name, force)
        },
}

var genModelCmd = &cobra.Command{
        Use:   "model [name]",
        Short: "Generate a new model",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
                name := args[0]
                force, _ := cmd.Flags().GetBool("force")
                return gen.GenerateModel(name, force)
        },
}

var genRoutesCmd = &cobra.Command{
    Use:   "routes [manifest]",
    Short: "Generate typed routes from a manifest (yaml)",
    Args:  cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        manifest := "routes.yml"
        if len(args) > 0 {
            manifest = args[0]
        }
        root := generateTarget
        if root == "" {
            var err error
            root, err = os.Getwd()
            if err != nil {
                return err
            }
        }
        outFlag, _ := cmd.Flags().GetString("out")
        if outFlag == "" {
            outFlag = filepath.Join(root, "app", "router", "routes_gen.go")
        }
        if err := rg.GenerateFromFile(manifest, outFlag); err != nil {
            return err
        }
        fmt.Println("created", outFlag)
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
        generateCmd.AddCommand(genRoutesCmd)
        generateCmd.AddCommand(genScaffoldCmd)
        genControllerCmd.Flags().Bool("force", false, "overwrite existing files")
        genModelCmd.Flags().Bool("force", false, "overwrite existing files")
        genRoutesCmd.Flags().String("out", "", "output file path for generated routes (default: <target>/app/router/routes_gen.go)")
        genScaffoldCmd.Flags().Bool("force", false, "overwrite existing files")
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
