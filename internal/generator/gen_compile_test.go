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

// readGoVersion reads the "go X.Y" line from repo's go.mod and returns the version
func readGoVersion(repo string) (string, error) {
	bm, err := os.ReadFile(filepath.Join(repo, "go.mod"))
	if err != nil {
		return "", err
	}
	s := string(bm)
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "go ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "go ")), nil
		}
	}
	// default to 1.20 if not present
	return "1.20", nil
}

func TestGeneratedModelCompilesAndRuns(t *testing.T) {
	repo := findRepoRoot()
	modName, err := readModuleName(repo)
	if err != nil {
		t.Fatalf("read module name: %v", err)
	}
	gov, err := readGoVersion(repo)
	if err != nil {
		gov = "1.20"
	}

	// create an isolated temporary module so tests don't modify repo/examples
	projDir := t.TempDir()
	uid := filepath.Base(projDir)
	moduleName := modName + "/examples/" + uid
	absRepo, _ := filepath.Abs(repo)
	goMod := "module " + moduleName + "\n\n" +
		"go " + gov + "\n\n" +
		"require " + modName + " v0.0.0\n\n" +
		"replace " + modName + " => " + absRepo + "\n"
	if err := os.WriteFile(filepath.Join(projDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	// build CLI (binary written into the temp project)
	bin := filepath.Join(projDir, "flow-cli")
	build := exec.Command("go", "build", "-o", bin, "./cmd/flow")
	build.Dir = repo
	if bout, err := build.CombinedOutput(); err != nil {
		// capture go env to help debug toolchain issues
		if envOut, e2 := exec.Command("go", "env").CombinedOutput(); e2 == nil {
			t.Fatalf("build cli failed: %v\noutput: %s\n--- go env ---\n%s", err, string(bout), string(envOut))
		}
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
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = projDir
	if out, err := tidy.CombinedOutput(); err != nil {
		if envOut, e2 := exec.Command("go", "env").CombinedOutput(); e2 == nil {
			t.Fatalf("go mod tidy failed: %v\n%s\n--- go env ---\n%s", err, string(out), string(envOut))
		}
		t.Fatalf("go mod tidy failed: %v\n%s", err, string(out))
	}

	// build and run
	cmd := exec.Command("go", "run", "main.go")
	cmd.Dir = projDir
	out, err := cmd.CombinedOutput()
	t.Logf("run output: %s", string(out))
	if err != nil {
		if envOut, e2 := exec.Command("go", "env").CombinedOutput(); e2 == nil {
			t.Fatalf("run failed: %v\n%s\n--- go env ---\n%s", err, string(out), string(envOut))
		}
		t.Fatalf("run failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "FOUND: compile-test-hello") {
		t.Fatalf("unexpected output: %s", string(out))
	}
}
