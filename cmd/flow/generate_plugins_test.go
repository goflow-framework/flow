package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	// ensure sample plugin is registered
	_ "github.com/undiegomejia/flow/pkg/plugins/sample"
)

func TestGenList_Formatted(t *testing.T) {
	buf := &bytes.Buffer{}
	// ensure json flag is false
	_ = genListCmd.Flags().Set("json", "false")
	genListCmd.SetOut(buf)
	if err := genListCmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
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
	// enable json flag
	_ = genListCmd.Flags().Set("json", "true")
	genListCmd.SetOut(buf)
	if err := genListCmd.Execute(); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	var arr []struct{
		Name string `json:"name"`
		Version string `json:"version"`
		Help string `json:"help"`
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
