package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "os/exec"
    "path/filepath"
)

func main() {
    var src string
    var outdir string
    var watch bool
    flag.StringVar(&src, "src", "app/assets", "source assets directory")
    flag.StringVar(&outdir, "outdir", "dist", "output directory for built assets")
    flag.BoolVar(&watch, "watch", false, "watch files and rebuild (dev mode)")
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

    args := []string{"--outdir=" + outPath}

    // find entry points: simple convention: js/*.js and css/*.css
    // we'll pass a glob to esbuild via npx which supports globs.
    jsGlob := filepath.Join(srcPath, "js", "*.js")
    cssGlob := filepath.Join(srcPath, "css", "*.css")
    args = append(args, jsGlob, cssGlob)

    if watch {
        args = append([]string{"esbuild", "--watch"}, args...)
    } else {
        args = append([]string{"esbuild"}, args...)
    }

    // Prefer local npx to ensure a usable esbuild is available.
    cmd := exec.Command("npx", args...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    fmt.Printf("Running: npx %v\n", args)
    if err := cmd.Run(); err != nil {
        log.Fatalf("esbuild failed: %v", err)
    }
}
