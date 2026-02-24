package main

import (
	"flag"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Simple doc generator: runs `go doc -all` for each package and writes a
// minimal HTML page wrapping the text output. Intended to be run in CI after
// checkout.

func main() {
	outDir := flag.String("out", ".ci/docs", "output directory for generated docs")
	flag.Parse()

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(2)
	}

	// get packages
	out, err := exec.Command("bash", "-lc", "go list ./...").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "go list failed: %v\n", err)
		os.Exit(2)
	}
	pkgs := strings.Fields(string(out))
	for _, pkg := range pkgs {
		// run go doc -all
		docout, err := exec.Command("go", "doc", "-all", pkg).Output()
		if err != nil {
			// continue on error but report
			fmt.Fprintf(os.Stderr, "go doc %s: %v\n", pkg, err)
		}
		safePkg := strings.ReplaceAll(pkg, "/", "-")
		od := filepath.Join(*outDir, safePkg)
		_ = os.MkdirAll(od, 0o755)
		txtPath := filepath.Join(od, "doc.txt")
		_ = os.WriteFile(txtPath, docout, 0o644)

		// write simple HTML wrapper
		htmlPath := filepath.Join(od, "doc.html")
		f, err := os.Create(htmlPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create html %s: %v\n", htmlPath, err)
			continue
		}
		title := html.EscapeString(pkg)
		body := html.EscapeString(string(docout))
		_, _ = f.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title=" + title + "</title></head><body>")
		_, _ = f.WriteString("<h1>" + title + "</h1><pre>")
		_, _ = f.WriteString(body)
		_, _ = f.WriteString("</pre></body></html>")
		f.Close()
	}
}
