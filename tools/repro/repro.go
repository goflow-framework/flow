package main

import (
	"fmt"
	"golang.org/x/tools/go/packages"
)

func main() {
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles}
	pkgs, err := packages.Load(cfg, "sync/atomic")
	if err != nil {
		fmt.Printf("packages.Load error: %v\n", err)
		return
	}
	for _, p := range pkgs {
		fmt.Printf("Pkg: %s\n", p.PkgPath)
		if len(p.Errors) > 0 {
			for _, e := range p.Errors {
				fmt.Printf("ERROR: %v\n", e)
			}
		} else {
			fmt.Printf("Loaded OK: files=%v\n", p.CompiledGoFiles)
		}
	}
}
