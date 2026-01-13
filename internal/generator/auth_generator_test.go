package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// TestCLI_GenerateAuth_Compiles generates auth into a new examples project,
// patches the placeholder model import in the generated controller to the
// repository module path, and runs a small main.go that imports the generated
// controllers and models to ensure the generated controller compiles.
func TestCLI_GenerateAuth_Compiles(t *testing.T) {
	repo := findRepoRoot()
	tmpProj, err := os.MkdirTemp(filepath.Join(repo, "examples"), "gen-compile-auth-*")
	if err != nil {
		t.Fatalf("mktemp proj dir: %v", err)
	}
	// clean up generated temp project to avoid leaving lots of gen-compile-auth-* folders
	defer func() { _ = os.RemoveAll(tmpProj) }()

	// build CLI
	bin := filepath.Join(tmpProj, "flow-cli")
	build := exec.Command("go", "build", "-o", bin, "./cmd/flow")
	build.Dir = repo
	if bout, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build cli failed: %v\noutput: %s", err, string(bout))
	}

	// generate auth into the project
	gen := exec.Command(bin, "generate", "auth", "--target", tmpProj)
	gen.Dir = repo
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generate auth failed: %v\n%s", err, string(out))
	}

	// compute module path and relative import prefix
	modName, err := readModuleName(repo)
	if err != nil {
		t.Fatalf("read module name: %v", err)
	}
	rel := strings.TrimPrefix(tmpProj, repo+string(os.PathSeparator))
	modelsImport := modName + "/" + filepath.ToSlash(filepath.Join(rel, "app", "models"))

	// patch generated controller to replace placeholder import path
	ctrlPath := filepath.Join(tmpProj, "app", "controllers", "auth_controller.go")
	b, err := os.ReadFile(ctrlPath)
	if err != nil {
		t.Fatalf("read generated controller: %v", err)
	}
	src := strings.Replace(string(b), "REPLACE_WITH_MODULE_PATH/app/models", modelsImport, 1)
	if err := os.WriteFile(ctrlPath, []byte(src), 0o644); err != nil {
		t.Fatalf("patch controller import: %v", err)
	}

	// patch generated middleware import to use the module path as well
	mwPath := filepath.Join(tmpProj, "app", "middleware", "auth.go")
	if mb, err := os.ReadFile(mwPath); err == nil {
		msrc := strings.Replace(string(mb), "REPLACE_WITH_MODULE_PATH/app/models", modelsImport, 1)
		if err := os.WriteFile(mwPath, []byte(msrc), 0o644); err != nil {
			t.Fatalf("patch middleware import: %v", err)
		}
	} else {
		t.Fatalf("read generated middleware: %v", err)
	}

	// write a main.go that imports controllers (blank import) and uses models.User
	controllersImport := modName + "/" + filepath.ToSlash(filepath.Join(rel, "app", "controllers"))
	middlewareImport := modName + "/" + filepath.ToSlash(filepath.Join(rel, "app", "middleware"))
	mainSrc := `package main

import (
    "context"
    "log"

    flow "` + modName + `/pkg/flow"
    orm "` + modName + `/internal/orm"
    models "` + modelsImport + `"
    _ "` + controllersImport + `"
    middleware "` + middlewareImport + `"
    _ "modernc.org/sqlite"
    "golang.org/x/crypto/bcrypt"
)

func main() {
    ctx := context.Background()
    adapter, err := orm.Connect("file::memory:?cache=shared")
    if err != nil {
        log.Fatal(err)
    }
    defer adapter.Close()

    app := flow.New("gen-compile-auth", flow.WithBun(adapter))
    if err := flow.AutoMigrate(ctx, app, (*models.User)(nil)); err != nil {
        log.Fatal(err)
    }
    pw, _ := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
    u := &models.User{Email: "admin@example.com", Password_hash: string(pw), Role: "admin"}
    if err := u.Save(ctx, app); err != nil {
        log.Fatal(err)
    }

    // ensure middleware helper symbol compiles
    _ = middleware.GetCurrentUser
}
`
	// write main.go
	if err := os.WriteFile(filepath.Join(tmpProj, "main.go"), []byte(mainSrc), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	// run
	cmd := exec.Command("go", "run", "main.go")
	cmd.Dir = tmpProj
	out, err := cmd.CombinedOutput()
	t.Logf("run output: %s", string(out))
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, string(out))
	}
}
