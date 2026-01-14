package main

import (
    "context"
    "flag"
    "log"
    "net/http"
    "os"
    "os/exec"
    "os/signal"
    "path/filepath"
    "strconv"
    "syscall"
    "time"
)

func runServer(dir string, port int, stop <-chan struct{}) error {
    fs := http.FileServer(http.Dir(dir))
    srv := &http.Server{Addr: ":" + strconv.Itoa(port), Handler: fs}

    go func() {
        <-stop
        ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
        defer cancel()
        _ = srv.Shutdown(ctx)
    }()

    log.Printf("serving %s on :%d", dir, port)
    return srv.ListenAndServe()
}

func main() {
    var src string
    var outdir string
    var watch bool
    var serve bool
    var port int
    flag.StringVar(&src, "src", "app/assets", "source assets directory")
    flag.StringVar(&outdir, "outdir", "dist", "output directory for built assets")
    flag.BoolVar(&watch, "watch", false, "watch files and rebuild (dev mode)")
    flag.BoolVar(&serve, "serve", false, "serve built assets over HTTP (dev)")
    flag.IntVar(&port, "port", 8000, "port to serve assets when --serve is used")
    flag.Parse()

    // resolve paths
    srcPath, err := filepath.Abs(src)
    if err != nil {
        log.Fatalf("resolve src: %v", err)
    }
    outPath, err := filepath.Abs(outdir)
    if err != nil {
        log.Fatalf("resolve outdir: %v", err)
    }

    // entrypoints
    jsGlob := filepath.Join(srcPath, "js", "*.js")
    cssGlob := filepath.Join(srcPath, "css", "*.css")

    // build argument list
    buildArgs := []string{"--outdir=" + outPath, jsGlob, cssGlob}

    // detect esbuild binary first, then fallback to npx
    var exe string
    var args []string
    if p, err := exec.LookPath("esbuild"); err == nil {
        exe = p
        if watch {
            args = append([]string{"--watch"}, buildArgs...)
        } else {
            args = buildArgs
        }
    } else if p, err := exec.LookPath("npx"); err == nil {
        exe = p
        if watch {
            args = append([]string{"esbuild", "--watch"}, buildArgs...)
        } else {
            args = append([]string{"esbuild"}, buildArgs...)
        }
    } else {
        log.Fatalf("esbuild not found: install esbuild or ensure npx is available in PATH")
    }

    // prepare command
    cmd := exec.Command(exe, args...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    // signal handling to cleanly stop watcher and server
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    defer signal.Stop(sigCh)

    stopServer := make(chan struct{})

    // If watch mode, start long-running process and optionally serve
    if watch {
        if err := cmd.Start(); err != nil {
            log.Fatalf("start esbuild: %v", err)
        }
        log.Printf("started %s %v", exe, args)

        // start serve if requested
        if serve {
            go func() {
                if err := runServer(outPath, port, stopServer); err != nil && err != http.ErrServerClosed {
                    log.Printf("asset server error: %v", err)
                }
            }()
        }

        // wait for termination
        <-sigCh
        log.Printf("shutting down")
        close(stopServer)
        // kill child if still running
        _ = cmd.Process.Signal(syscall.SIGINT)
        done := make(chan error)
        go func() { done <- cmd.Wait() }()
        select {
        case <-time.After(3 * time.Second):
            _ = cmd.Process.Kill()
        case <-done:
        }
        return
    }

    // non-watch: run build once
    log.Printf("running %s %v", exe, args)
    if err := cmd.Run(); err != nil {
        log.Fatalf("esbuild failed: %v", err)
    }

    if serve {
        // serve built files until signal
        go func() {
            if err := runServer(outPath, port, stopServer); err != nil && err != http.ErrServerClosed {
                log.Fatalf("asset server error: %v", err)
            }
        }()
        <-sigCh
        close(stopServer)
        // give server a moment to shutdown
        time.Sleep(200 * time.Millisecond)
    }
}
