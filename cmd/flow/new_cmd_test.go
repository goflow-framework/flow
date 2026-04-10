package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// scaffoldApp calls runNew via the cobra command tree into a temporary
// directory and returns the project root path.
func scaffoldApp(t *testing.T, name string) string {
	t.Helper()
	tmp := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	root := &cobra.Command{Use: "flow"}
	root.AddCommand(newCmd)
	root.SetArgs([]string{"new", name})
	if err := root.Execute(); err != nil {
		t.Fatalf("flow new %s: %v", name, err)
	}
	return filepath.Join(tmp, name)
}

func TestRunNew_CreatesMainTestFile(t *testing.T) {
	root := scaffoldApp(t, "myapp")
	path := filepath.Join(root, "cmd", "myapp", "main_test.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("main_test.go not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "package main") {
		t.Error("main_test.go: expected 'package main'")
	}
	if !strings.Contains(content, "func TestSmoke") {
		t.Error("main_test.go: expected 'func TestSmoke'")
	}
}

func TestRunNew_CreatesInternalTestFile(t *testing.T) {
	root := scaffoldApp(t, "myapp2")
	path := filepath.Join(root, "internal", "myapp2_test.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("internal/<app>_test.go not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "package internal_test") {
		t.Error("internal test file: expected 'package internal_test'")
	}
	if !strings.Contains(content, "func TestIntegration") {
		t.Error("internal test file: expected 'func TestIntegration'")
	}
}

func TestRunNew_TestFileContainsAppName(t *testing.T) {
	const appName = "coolapp"
	root := scaffoldApp(t, appName)

	smokeData, err := os.ReadFile(filepath.Join(root, "cmd", appName, "main_test.go"))
	if err != nil {
		t.Fatalf("main_test.go not created: %v", err)
	}
	if !strings.Contains(string(smokeData), appName) {
		t.Errorf("main_test.go: expected app name %q in content", appName)
	}

	intData, err := os.ReadFile(filepath.Join(root, "internal", appName+"_test.go"))
	if err != nil {
		t.Fatalf("internal test file not created: %v", err)
	}
	if !strings.Contains(string(intData), appName) {
		t.Errorf("internal test file: expected app name %q in content", appName)
	}
}

func TestRunNew_AllExpectedFilesExist(t *testing.T) {
	root := scaffoldApp(t, "fullapp")
	expected := []string{
		"go.mod",
		filepath.Join("cmd", "fullapp", "main.go"),
		filepath.Join("cmd", "fullapp", "main_test.go"),
		filepath.Join("internal", "fullapp_test.go"),
		".env.example",
		"Makefile",
	}
	for _, rel := range expected {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("expected file missing: %s", rel)
		}
	}
}

func TestRunNew_RejectsExistingDirectory(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(tmp)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// pre-create the directory
	if err := os.Mkdir("taken", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	root := &cobra.Command{Use: "flow"}
	root.AddCommand(newCmd)
	root.SetArgs([]string{"new", "taken"})
	if err := root.Execute(); err == nil {
		t.Error("expected error when target directory already exists, got nil")
	}
}
