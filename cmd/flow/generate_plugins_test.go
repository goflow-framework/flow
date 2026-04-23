package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	// ensure sample plugin is registered
	_ "github.com/goflow-framework/flow/pkg/plugins/sample"

	gen "github.com/goflow-framework/flow/internal/generator"
	"github.com/goflow-framework/flow/pkg/plugins"
)

func TestGenList_Formatted(t *testing.T) {
	buf := &bytes.Buffer{}
	root := &cobra.Command{Use: "app"}
	root.AddCommand(generateCmd)
	root.SetOut(buf)
	root.SetArgs([]string{"generate", "plugins"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute failed: %v; out=%s", err, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "samplegen") {
		t.Fatalf("expected samplegen in output, got: %s", out)
	}
	if !strings.Contains(out, "0.0.1") {
		t.Fatalf("expected version in output, got: %s", out)
	}
}

func TestGenList_JSON(t *testing.T) {
	buf := &bytes.Buffer{}
	root := &cobra.Command{Use: "app"}
	root.AddCommand(generateCmd)
	root.SetOut(buf)
	root.SetArgs([]string{"generate", "plugins", "--format", "json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute failed: %v; out=%s", err, buf.String())
	}
	var arr []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Help    string `json:"help"`
	}
	if err := json.Unmarshal(buf.Bytes(), &arr); err != nil {
		t.Fatalf("unmarshal json failed: %v; out=%s", err, buf.String())
	}
	if len(arr) == 0 {
		t.Fatalf("expected at least one generator in json output")
	}
	found := false
	for _, it := range arr {
		if it.Name == "samplegen" {
			found = true
			if it.Version != "0.0.1" {
				t.Fatalf("unexpected version for samplegen: %s", it.Version)
			}
		}
	}
	if !found {
		t.Fatalf("samplegen not found in json output: %s", buf.String())
	}
}

func TestGenList_NoPlugins(t *testing.T) {
	// snapshot current generators and restore after test
	saved := map[string]interface{}{}
	for _, name := range gen.ListRegisteredGenerators() {
		// use the public plugins API to fetch generator implementations
		g := plugins.GetGenerator(name)
		if g != nil {
			saved[name] = g
		}
	}
	// clear registry
	plugins.Reset()
	defer func() {
		// restore saved generators
		for _, v := range saved {
			if g, ok := v.(plugins.GeneratorPlugin); ok {
				_ = plugins.RegisterGenerator(g)
			}
		}
	}()

	buf := &bytes.Buffer{}
	root := &cobra.Command{Use: "app"}
	root.AddCommand(generateCmd)
	root.SetOut(buf)
	root.SetArgs([]string{"generate", "plugins"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute failed: %v; out=%s", err, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "no generator plugins registered") {
		t.Fatalf("expected no plugins message, got: %s", out)
	}
}
