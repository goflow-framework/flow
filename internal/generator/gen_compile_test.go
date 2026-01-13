package generator

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// readModuleName reads the module name from repo's go.mod
func readModuleName(repo string) (string, error) {
	bm, err := os.ReadFile(filepath.Join(repo, "go.mod"))
	if err != nil {
		return "", err
	}
	s := string(bm)
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", nil
}

func TestGeneratedModelCompilesAndRuns(t *testing.T) {
	repo := findRepoRoot()
	modName, err := readModuleName(repo)
	if err != nil {
		t.Fatalf("read module name: %v", err)
	}

	// create a project dir under examples so it is inside the module
	projDir, err := os.MkdirTemp(filepath.Join(repo, "examples"), "gen-compile-*")
	if err != nil {
		t.Fatalf("mktemp proj dir: %v", err)
	}
	// remove the generated temp project when the test finishes to avoid
	// leaving many gen-compile-* folders under examples/ from repeated runs.
	defer func() { _ = os.RemoveAll(projDir) }()

	// build CLI
	bin := filepath.Join(projDir, "flow-cli")
	build := exec.Command("go", "build", "-o", bin, "./cmd/flow")
	build.Dir = repo
	if bout, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build cli failed: %v\noutput: %s", err, string(bout))
	}

	// generate model into projDir
	gen := exec.Command(bin, "generate", "model", "Post", "title:string", "--target", projDir)
	gen.Dir = repo
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generate model failed: %v\n%s", err, string(out))
	}

	// create main.go that uses the generated model's Save/Delete
	rel := strings.TrimPrefix(projDir, repo+string(os.PathSeparator))
	modelsImport := modName + "/" + filepath.ToSlash(filepath.Join(rel, "app", "models"))
	mainSrc := `package main

import (
    "context"
    "fmt"
    "log"

    flow "` + modName + `/pkg/flow"
    orm "` + modName + `/internal/orm"
    models "` + modelsImport + `"
    _ "modernc.org/sqlite"
)

func main() {
    ctx := context.Background()
    adapter, err := orm.Connect("file::memory:?cache=shared")
    if err != nil {
        log.Fatalf("connect: %v", err)
    }
    defer adapter.Close()

    app := flow.New("gen-compile", flow.WithBun(adapter))
    if err := flow.AutoMigrate(ctx, app, (*models.Post)(nil)); err != nil {
        log.Fatalf("migrate: %v", err)
    }

    p := &models.Post{Title: "compile-test-hello"}
    if err := p.Save(ctx, app); err != nil {
        log.Fatalf("save: %v", err)
    }
    var got models.Post
    if err := flow.FindByPK(ctx, app, &got, p.ID); err != nil {
        log.Fatalf("find: %v", err)
    }
    fmt.Println("FOUND:", got.Title)

    if err := p.Delete(ctx, app); err != nil {
        log.Fatalf("delete: %v", err)
    }
}
`

	if err := os.WriteFile(filepath.Join(projDir, "main.go"), []byte(mainSrc), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	// build and run
	cmd := exec.Command("go", "run", "main.go")
	cmd.Dir = projDir
	out, err := cmd.CombinedOutput()
	t.Logf("run output: %s", string(out))
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if !strings.Contains(string(out), "FOUND: compile-test-hello") {
		t.Fatalf("unexpected output: %s", string(out))
	}
}
