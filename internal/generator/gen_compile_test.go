package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedModelCompilesAndRuns(t *testing.T) {
	repo := findRepoRoot()
	modName, err := readModuleName(repo)
	if err != nil {
		t.Fatalf("read module name: %v", err)
	}
	// create an isolated temporary module so tests don't modify repo/examples
	projDir := t.TempDir()
	uid := filepath.Base(projDir)
	moduleName := modName + "/examples/" + uid
	if err := WriteTempGoMod(projDir, moduleName, false); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	// build CLI (binary written into the temp project)
	bin := filepath.Join(projDir, "flow-cli")
	if bout, err := RunGoCombined(repo, "build", "-o", bin, "./cmd/flow"); err != nil {
		t.Fatalf("build cli failed: %v\noutput: %s", err, string(bout))
	}

	// generate model into projDir
	gen := exec.Command(bin, "generate", "model", "Post", "title:string", "--target", projDir)
	gen.Dir = repo
	if out, err := gen.CombinedOutput(); err != nil {
		if envOut, e2 := exec.Command("go", "env").CombinedOutput(); e2 == nil {
			t.Fatalf("generate model failed: %v\n%s\n--- go env ---\n%s", err, string(out), string(envOut))
		}
		t.Fatalf("generate model failed: %v\n%s", err, string(out))
	}

	// create main.go that uses the generated model's Save/Delete
	modelsImport := moduleName + "/app/models"
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

	// tidy deps before running so the temp module resolves local repo packages
	if out, err := RunCmdCombined(projDir, "go", "mod", "tidy"); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, string(out))
	}

	// build and run using RunGoCombined so tests use a local GOMODCACHE
	out, err := RunGoCombined(projDir, "run", "main.go")
	t.Logf("run output: %s", string(out))
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "FOUND: compile-test-hello") {
		t.Fatalf("unexpected output: %s", string(out))
	}
}
