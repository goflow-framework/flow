package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI_GenerateScaffold_SkipMigrations(t *testing.T) {
	repo := findRepoRoot()
	tmp := t.TempDir()

	// build CLI
	bin := filepath.Join(tmp, "flow-cli")
	build := exec.Command("go", "build", "-o", bin, "./cmd/flow")
	build.Dir = repo
	if bout, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build cli failed: %v\noutput: %s", err, string(bout))
	}

	// run scaffold generator with --skip-migrations
	cmd := exec.Command(bin, "generate", "scaffold", "post", "title:string", "--target", tmp, "--skip-migrations")
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	t.Logf("cmd output: %s", string(out))
	if err != nil {
		t.Fatalf("cli generate scaffold failed: %v", err)
	}

	// migrations dir should not exist
	migDir := filepath.Join(tmp, "db", "migrate")
	if _, err := os.Stat(migDir); !os.IsNotExist(err) {
		t.Fatalf("expected migrations dir to be absent when --skip-migrations is used, found: %v", err)
	}
}

func TestCLI_GenerateScaffold_NoViews(t *testing.T) {
	repo := findRepoRoot()
	tmp := t.TempDir()

	// build CLI
	bin := filepath.Join(tmp, "flow-cli")
	build := exec.Command("go", "build", "-o", bin, "./cmd/flow")
	build.Dir = repo
	if bout, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build cli failed: %v\noutput: %s", err, string(bout))
	}

	// run scaffold generator with --no-views
	cmd := exec.Command(bin, "generate", "scaffold", "post", "title:string", "--target", tmp, "--no-views")
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	t.Logf("cmd output: %s", string(out))
	if err != nil {
		t.Fatalf("cli generate scaffold failed: %v", err)
	}

	// views dir should not exist (or be empty)
	viewsDir := filepath.Join(tmp, "app", "views", "post")
	if _, err := os.Stat(viewsDir); !os.IsNotExist(err) {
		// if exists, ensure no view files
		entries, rerr := os.ReadDir(viewsDir)
		if rerr == nil && len(entries) > 0 {
			t.Fatalf("expected no views when --no-views is used, found %d files", len(entries))
		}
	}
}

func TestCLI_GenerateModel_ForceOverwrite(t *testing.T) {
	repo := findRepoRoot()
	tmp := t.TempDir()

	// build CLI
	bin := filepath.Join(tmp, "flow-cli")
	build := exec.Command("go", "build", "-o", bin, "./cmd/flow")
	build.Dir = repo
	if bout, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build cli failed: %v\noutput: %s", err, string(bout))
	}

	// create an existing model file to simulate conflict
	modelDir := filepath.Join(tmp, "app", "models")
	if err := os.MkdirAll(modelDir, 0o750); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	modelPath := filepath.Join(modelDir, "post.go")
	if err := os.WriteFile(modelPath, []byte("// old"), 0o644); err != nil {
		t.Fatalf("write old model failed: %v", err)
	}

	// run generator without --force: should error
	cmd := exec.Command(bin, "generate", "model", "Post", "title:string", "--target", tmp)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	t.Logf("cmd output: %s", string(out))
	if err == nil {
		t.Fatalf("expected generator to fail when file exists and --force not provided")
	}

	// run generator with --force: should succeed and overwrite file
	cmd2 := exec.Command(bin, "generate", "model", "Post", "title:string", "--target", tmp, "--force")
	cmd2.Dir = repo
	out2, err2 := cmd2.CombinedOutput()
	t.Logf("cmd output: %s", string(out2))
	if err2 != nil {
		t.Fatalf("expected generator to succeed with --force: %v", err2)
	}

	// check new file contains bun tag
	b, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("read generated model: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `bun:"title"`) {
		t.Fatalf("generated model missing bun tag for title after --force: %s", s)
	}
}
