package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCLI_GenerateAdmin_Smoke generates admin scaffolding into a project under
// repo/examples/<uid>, patches a mount call, and runs a small main program that
// mounts the admin routes and performs an HTTP GET to /admin/posts to assert
// the handler is reachable. Placing the generated project under the repo
// avoids network module lookups by the go tool.
func TestCLI_GenerateAdmin_Smoke(t *testing.T) {
	repo := findRepoRoot()
	modName, err := readModuleName(repo)
	if err != nil {
		t.Fatalf("read module name: %v", err)
	}

	// create a repo-local examples dir and a unique project dir under it
	tmpBase := filepath.Join(repo, "examples")
	if err := os.MkdirAll(tmpBase, 0o755); err != nil {
		t.Fatalf("mkdir examples dir: %v", err)
	}
	uid := filepath.Base(t.TempDir())
	proj := filepath.Join(tmpBase, uid)
	// ensure we clean up generated project after the test
	defer func() { _ = os.RemoveAll(proj) }()
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatalf("mkdir proj dir: %v", err)
	}

	moduleName := modName + "/examples/" + uid

	// do NOT write a go.mod in the generated project; we'll run the generated
	// main from the repo root so the repo's go.mod is used (avoids nested
	// module resolution issues).

	// build CLI binary (placed in proj to avoid polluting repo root)
	bin := filepath.Join(proj, "flow-cli")
	if out, err := RunCmdCombined(repo, "go", "build", "-o", bin, "./cmd/flow"); err != nil {
		t.Fatalf("build cli failed: %v\noutput: %s", err, string(out))
	}

	// generate admin scaffolding for 'posts'
	if out, err := RunCmdCombined(repo, bin, "generate", "admin", "posts", "--target", proj); err != nil {
		t.Fatalf("generate admin failed: %v\n%s", err, string(out))
	}

	// patch generated controller to use app.SetRouter(r.Handler()) instead of app.Mount(r)
	ctrlPath := filepath.Join(proj, "app", "controllers", "admin", "posts_admin_controller.go")
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

	if err := os.WriteFile(filepath.Join(proj, "main.go"), []byte(mainSrc), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	// run the generated main from the repo root so imports resolve against the
	// repository module (repo's go.mod)
	relMain := filepath.Join("./examples", uid, "main.go")
	out, err := RunCmdCombined(repo, "go", "run", relMain)
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
		if out, err := RunCmdCombined(repo, "go", "build", "-o", bin, "./cmd/flow"); err != nil {
			if envOut, _ := RunCmdCombined(repo, "go", "env"); envOut != nil {
				t.Fatalf("build generator failed: %v\n%s\n--- go env ---\n%s", err, string(out), string(envOut))
			}
			t.Fatalf("build generator failed: %v\n%s", err, string(out))
		}
	}

	// tidy before running generator to ensure local replacements are taken into account
	if out, err := RunCmdCombined(proj, "go", "mod", "tidy"); err != nil {
		if envOut, _ := RunCmdCombined(repo, "go", "env"); envOut != nil {
			t.Fatalf("go mod tidy failed: %v\n%s\n--- go env ---\n%s", err, string(out), string(envOut))
		}
		t.Fatalf("go mod tidy failed: %v\n%s", err, string(out))
	}

	// run the smoke generator
	if out, err := RunCmdCombined(repo, bin, "generate", "admin", "dashboard", "--target", proj); err != nil {
		if envOut, _ := RunCmdCombined(repo, "go", "env"); envOut != nil {
			t.Fatalf("admin generate failed: %v\n%s\n--- go env ---\n%s", err, string(out), string(envOut))
		}
		t.Fatalf("admin generate failed: %v\n%s", err, string(out))
	}
}
