package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// findRepoRoot helper moved to testutil.go

func TestCLI_GenerateModel_WritesBunTaggedModel(t *testing.T) {
	repo := findRepoRoot()
	tmp := t.TempDir()

	// build the CLI binary into the temp dir then run it to avoid go run
	bin := filepath.Join(tmp, "flow-cli")
	if bout, err := RunGoCombined(repo, "build", "-o", bin, "./cmd/flow"); err != nil {
		t.Fatalf("build cli failed: %v\noutput: %s", err, string(bout))
	}

	// run generated binary: generate model into tmp target
	cmd := exec.Command(bin, "generate", "model", "Post", "title:string", "--target", tmp)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	t.Logf("cmd output: %s", string(out))
	if err != nil {
		t.Fatalf("go run cmd/flow failed: %v", err)
	}

	// check file exists and contains bun tag
	gotPath := filepath.Join(tmp, "app", "models", "post.go")
	b, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("read generated model: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `bun:"title"`) {
		t.Fatalf("generated model missing bun tag for title: %s", s)
	}
	if !strings.Contains(s, "type Post struct") {
		t.Fatalf("generated model missing struct declaration: %s", s)
	}
}

func TestCLI_GenerateScaffold_CreatesFilesAndMigration(t *testing.T) {
	repo := findRepoRoot()
	tmp := t.TempDir()

	// build CLI
	bin := filepath.Join(tmp, "flow-cli")
	if bout, err := RunGoCombined(repo, "build", "-o", bin, "./cmd/flow"); err != nil {
		t.Fatalf("build cli failed: %v\noutput: %s", err, string(bout))
	}

	// run scaffold generator for resource 'post'
	cmd := exec.Command(bin, "generate", "scaffold", "post", "title:string", "published_at:datetime", "--target", tmp)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	t.Logf("cmd output: %s", string(out))
	if err != nil {
		t.Fatalf("cli generate scaffold failed: %v", err)
	}

	// assert controller
	ctrlPath := filepath.Join(tmp, "app", "controllers", "post_controller.go")
	if _, err := os.Stat(ctrlPath); err != nil {
		t.Fatalf("controller not created: %v", err)
	}

	// assert model
	modelPath := filepath.Join(tmp, "app", "models", "post.go")
	b, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatalf("read generated model: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, `bun:"title"`) {
		t.Fatalf("generated model missing bun tag for title: %s", s)
	}
	if !strings.Contains(s, "PublishedAt") && !strings.Contains(s, "published_at") {
		t.Fatalf("generated model missing PublishedAt field: %s", s)
	}

	// assert views
	views := []string{"index.html", "show.html", "new.html", "edit.html"}
	for _, v := range views {
		p := filepath.Join(tmp, "app", "views", "post", v)
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("view %s not created: %v", v, err)
		}
	}

	// assert migrations (up/down) exist and up contains table and columns
	migDir := filepath.Join(tmp, "db", "migrate")
	entries, err := os.ReadDir(migDir)
	if err != nil {
		t.Fatalf("migrations dir not found: %v", err)
	}
	var upFile string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			upFile = filepath.Join(migDir, e.Name())
		}
	}
	if upFile == "" {
		t.Fatalf("no up migration found in %s", migDir)
	}
	mb, err := os.ReadFile(upFile)
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	ms := string(mb)
	if !strings.Contains(ms, "CREATE TABLE IF NOT EXISTS posts") {
		t.Fatalf("up migration missing CREATE TABLE posts: %s", ms)
	}
	if !strings.Contains(ms, "title TEXT") {
		t.Fatalf("up migration missing title column: %s", ms)
	}
	if !strings.Contains(ms, "published_at DATETIME") {
		t.Fatalf("up migration missing published_at column: %s", ms)
	}
}
