package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCLI_GenerateAdmin_Smoke generates admin scaffolding into a temporary
// project, patches the generated mount helper to use app.SetRouter, and runs
// a small program that mounts the admin routes and performs an HTTP GET to
// /admin/posts to assert the handler is reachable.
func TestCLI_GenerateAdmin_Smoke(t *testing.T) {
	repo := findRepoRoot()
	modName, err := readModuleName(repo)
	if err != nil {
		t.Fatalf("read module name: %v", err)
	}

	tmpProj := t.TempDir()
	uid := filepath.Base(tmpProj)
	// Use an independent module name (not a subpath of the repo module) to
	// avoid Go module resolution treating the generated package as part of the
	// main repo module. This prevents `go mod tidy` from attempting to fetch
	// packages from the parent module.
	moduleName := "example.com/" + uid
	goMod := "module " + moduleName + "\n\n" +
		"go 1.20\n\n" +
		"require " + modName + " v0.0.0\n\n" +
		"replace " + modName + " => " + repo + "\n" +
		"replace " + moduleName + " => .\n"
	if err := os.WriteFile(filepath.Join(tmpProj, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	// build CLI binary
	bin := filepath.Join(tmpProj, "flow-cli")
	build := exec.Command("go", "build", "-o", bin, "./cmd/flow")
	build.Dir = repo
	if bout, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build cli failed: %v\noutput: %s", err, string(bout))
	}

	// generate admin scaffolding for 'posts'
	gen := exec.Command(bin, "generate", "admin", "posts", "--target", tmpProj)
	gen.Dir = repo
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generate admin failed: %v\n%s", err, string(out))
	}

	// patch generated controller to use app.SetRouter(r.Handler()) instead of app.Mount(r)
	ctrlPath := filepath.Join(tmpProj, "app", "controllers", "admin", "posts_admin_controller.go")
	b, err := os.ReadFile(ctrlPath)
	if err != nil {
		t.Fatalf("read generated admin controller: %v", err)
	}
	src := strings.Replace(string(b), "app.Mount(r)", "app.SetRouter(r.Handler())", 1)
	if err := os.WriteFile(ctrlPath, []byte(src), 0o644); err != nil {
		t.Fatalf("patch admin controller mount call: %v", err)
	}

	// write a main.go that mounts the generated admin routes and issues a GET to /admin/posts
	controllersImport := moduleName + "/app/controllers"
	mainSrc := `package main

import (
    "fmt"
    "net/http"
    "net/http/httptest"

    flow "` + modName + `/pkg/flow"
    controllers "` + controllersImport + `"
)

func main() {
    app := flow.New("gen-compile-admin")
    // point views to the generated views directory
    app.Views = flow.NewViewManager("app/views")

    controllers.MountAdminPostsRoutes(app)

    srv := httptest.NewServer(app.Handler())
    defer srv.Close()

    resp, err := http.Get(srv.URL + "/admin/posts")
    if err != nil {
        panic(err)
    }
    fmt.Printf("STATUS:%d\n", resp.StatusCode)
}
`

	if err := os.WriteFile(filepath.Join(tmpProj, "main.go"), []byte(mainSrc), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	// tidy and run
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = tmpProj
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, string(out))
	}

	cmd := exec.Command("go", "run", "main.go")
	cmd.Dir = tmpProj
	out, err := cmd.CombinedOutput()
	t.Logf("run output: %s", string(out))
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "STATUS:200") {
		t.Fatalf("unexpected output, expected STATUS:200, got: %s", string(out))
	}
}

func TestAdminGeneratorSmoke(t *testing.T) {
	repo := findRepoRoot()
	gov, err := readGoVersion(repo)
	if err != nil {
		gov = "1.20"
	}
	absRepo, _ := filepath.Abs(repo)

	proj := t.TempDir()
	uid := filepath.Base(proj)
	moduleName := "example.com/" + uid
	absProj, _ := filepath.Abs(proj)

	goMod := "module " + moduleName + "\n\n" +
		"go " + gov + "\n\n" +
		"require " + "" + "\n\n" +
		"replace " + "github.com/undiegomejia/flow => " + absRepo + "\n" +
		"replace " + moduleName + " => " + absProj + "\n"
	if err := os.WriteFile(filepath.Join(proj, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	// run generator
	bin := filepath.Join(repo, "bin", "flow-gen")
	if _, err := os.Stat(bin); os.IsNotExist(err) {
		// try building the generator
		b := exec.Command("go", "build", "-o", bin, "./cmd/flow")
		b.Dir = repo
		if out, err := b.CombinedOutput(); err != nil {
			if envOut, e2 := exec.Command("go", "env").CombinedOutput(); e2 == nil {
				t.Fatalf("build generator failed: %v\n%s\n--- go env ---\n%s", err, string(out), string(envOut))
			}
			t.Fatalf("build generator failed: %v\n%s", err, string(out))
		}
	}

	// tidy before running generator to ensure local replacements are taken into account
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = proj
	if out, err := tidy.CombinedOutput(); err != nil {
		if envOut, e2 := exec.Command("go", "env").CombinedOutput(); e2 == nil {
			t.Fatalf("go mod tidy failed: %v\n%s\n--- go env ---\n%s", err, string(out), string(envOut))
		}
		t.Fatalf("go mod tidy failed: %v\n%s", err, string(out))
	}

	// run the smoke generator
	ren := exec.Command(bin, "generate", "admin", "dashboard", "--target", proj)
	ren.Dir = repo
	if out, err := ren.CombinedOutput(); err != nil {
		if envOut, e2 := exec.Command("go", "env").CombinedOutput(); e2 == nil {
			t.Fatalf("admin generate failed: %v\n%s\n--- go env ---\n%s", err, string(out), string(envOut))
		}
		t.Fatalf("admin generate failed: %v\n%s", err, string(out))
	}
}
