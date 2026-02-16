package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteTempGoMod_WritesGoVersionAndAbsoluteReplace(t *testing.T) {
	repo := findRepoRoot()
	modName, err := readModuleName(repo)
	if err != nil {
		t.Fatalf("read module name: %v", err)
	}

	proj, _ := TempModule(t)

	b, err := os.ReadFile(filepath.Join(proj, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	s := string(b)

	// Should contain a go directive
	if !strings.Contains(s, "\ngo ") {
		t.Fatalf("go.mod missing go directive: %s", s)
	}

	// Should contain an absolute replace to the repo module path
	absRepo, _ := filepath.Abs(repo)
	replaceLine := "replace " + modName + " => " + absRepo
	if !strings.Contains(s, replaceLine) {
		t.Fatalf("go.mod missing absolute replace; expected %q in %s", replaceLine, s)
	}
}
