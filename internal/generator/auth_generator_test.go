package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGenerateAuthWithOptions_CreatesFiles(t *testing.T) {
	tmp := t.TempDir()
	created, err := GenerateAuthWithOptions(tmp, GenOptions{Force: true})
	if err != nil {
		t.Fatalf("GenerateAuthWithOptions failed: %v", err)
	}
	expected := []string{
		filepath.Join(tmp, "app", "models", "user.go"),
		filepath.Join(tmp, "app", "controllers", "auth_controller.go"),
		filepath.Join(tmp, "app", "views", "auth", "login.html"),
		filepath.Join(tmp, "app", "middleware", "auth.go"),
		filepath.Join(tmp, "app", "auth", "README.md"),
	}
	for _, p := range expected {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected generated file %s to exist: %v", p, err)
		}
	}
	_ = created
}

func TestCLI_GenerateAuth_CreatesFiles(t *testing.T) {
	repo := findRepoRoot()
	tmp := t.TempDir()

	// build CLI
	bin := filepath.Join(tmp, "flow-cli")
	build := exec.Command("go", "build", "-o", bin, "./cmd/flow")
	build.Dir = repo
	if bout, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build cli failed: %v\noutput: %s", err, string(bout))
	}

	// run generated binary: generate auth into tmp target
	cmd := exec.Command(bin, "generate", "auth", "--target", tmp)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	t.Logf("cmd output: %s", string(out))
	if err != nil {
		t.Fatalf("cli generate auth failed: %v", err)
	}

	// check expected files exist
	paths := []string{
		filepath.Join(tmp, "app", "models", "user.go"),
		filepath.Join(tmp, "app", "controllers", "auth_controller.go"),
		filepath.Join(tmp, "app", "views", "auth", "login.html"),
		filepath.Join(tmp, "app", "middleware", "auth.go"),
		filepath.Join(tmp, "app", "auth", "README.md"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("expected generated file %s to exist: %v", p, err)
		}
	}
}
