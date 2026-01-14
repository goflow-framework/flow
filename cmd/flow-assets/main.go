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

    // After a non-watch build, generate a simple manifest mapping original
    // asset paths to fingerprinted filenames. This allows templates to refer
    // to logical asset paths and pick up fingerprinted names for cache busting.
    // We skip manifest generation when outdir is empty or not writable.
    if err := generateManifest(outPath); err != nil {
        log.Printf("warning: manifest generation failed: %v", err)
    }
}

// generateManifest walks outdir, computes a short SHA1 for each file and
// renames the file to include the hash before the extension, then writes
// manifest.json mapping original -> fingerprinted path. Existing
// manifest.json is overwritten.
func generateManifest(outdir string) error {
    // local imports
    // compute map
    manifest := make(map[string]string)
    err := filepath.Walk(outdir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if info.IsDir() {
            return nil
        }
        rel, err := filepath.Rel(outdir, path)
        if err != nil {
            return err
        }
        if rel == "manifest.json" {
            return nil
        }
        // read file
        data, err := os.ReadFile(path)
        if err != nil {
            return err
        }
        // compute short sha1
        h := sha1.Sum(data)
        short := fmt.Sprintf("%x", h)[:8]
        ext := filepath.Ext(rel)
        base := strings.TrimSuffix(rel, ext)
        newRel := base + "." + short + ext
        newPath := filepath.Join(outdir, newRel)
        // ensure dir exists
        if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
            return err
        }
        // rename (overwrite if exists)
        if err := os.Rename(path, newPath); err != nil {
            return err
        }
        // store with forward slashes for manifest consistency
        manifest[filepath.ToSlash(rel)] = filepath.ToSlash(newRel)
        return nil
    })
    if err != nil {
        return err
    }
    // write manifest.json
    outb, err := json.MarshalIndent(manifest, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(filepath.Join(outdir, "manifest.json"), outb, 0o644)
}
