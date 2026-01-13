package generator

import (
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "testing"
)

func TestGenerateAdminWithOptions_CreatesFiles(t *testing.T) {
    tmp := t.TempDir()
    // call generator directly
    created, err := GenerateAdminWithOptions(tmp, "post", GenOptions{Force: true}, "title:string")
    if err != nil {
        t.Fatalf("GenerateAdminWithOptions failed: %v", err)
    }
    t.Logf("created list: %v", created)
    // list admin views dir for debugging
    adViews := filepath.Join(tmp, "app", "views", "admin")
    if entries, err := os.ReadDir(adViews); err == nil {
        var names []string
        for _, e := range entries {
            names = append(names, e.Name())
        }
        t.Logf("admin views entries: %v", names)
    } else {
        t.Logf("admin views readdir error: %v", err)
    }
    // expected files
    expected := []string{
        filepath.Join(tmp, "app", "controllers", "admin", "post_admin_controller.go"),
        filepath.Join(tmp, "app", "views", "admin", "post", "index.html"),
        filepath.Join(tmp, "app", "views", "admin", "post", "show.html"),
        filepath.Join(tmp, "app", "views", "admin", "post", "new.html"),
        filepath.Join(tmp, "app", "views", "admin", "post", "edit.html"),
        filepath.Join(tmp, "app", "views", "admin", "layouts", "admin.html"),
        filepath.Join(tmp, "app", "assets", "admin", "admin.css"),
        filepath.Join(tmp, "app", "admin", "README.md"),
    }
    for _, p := range expected {
        if _, err := os.Stat(p); err != nil {
            t.Fatalf("expected generated file %s to exist: %v", p, err)
        }
    }
    // basic content checks
    ctrl := filepath.Join(tmp, "app", "controllers", "admin", "post_admin_controller.go")
    b, err := os.ReadFile(ctrl)
    if err != nil {
        t.Fatalf("read controller: %v", err)
    }
    s := string(b)
    if !strings.Contains(s, "/admin/post") {
        t.Fatalf("controller missing admin base route: %s", s)
    }
    if !strings.Contains(strings.Join(created, "\n"), ctrl) {
        t.Fatalf("returned created list missing controller path: %v", created)
    }
}

func TestCLI_GenerateAdmin_CreatesFiles(t *testing.T) {
    repo := findRepoRoot()
    tmp := t.TempDir()

    // build CLI
    bin := filepath.Join(tmp, "flow-cli")
    build := exec.Command("go", "build", "-o", bin, "./cmd/flow")
    build.Dir = repo
    if bout, err := build.CombinedOutput(); err != nil {
        t.Fatalf("build cli failed: %v\noutput: %s", err, string(bout))
    }

    // run generated binary: generate admin into tmp target
    cmd := exec.Command(bin, "generate", "admin", "post", "title:string", "--target", tmp)
    cmd.Dir = repo
    out, err := cmd.CombinedOutput()
    t.Logf("cmd output: %s", string(out))
    if err != nil {
        t.Fatalf("cli generate admin failed: %v", err)
    }

    // check expected files exist
    paths := []string{
        filepath.Join(tmp, "app", "controllers", "admin", "post_admin_controller.go"),
        filepath.Join(tmp, "app", "views", "admin", "post", "index.html"),
        filepath.Join(tmp, "app", "views", "admin", "post", "show.html"),
        filepath.Join(tmp, "app", "views", "admin", "post", "new.html"),
        filepath.Join(tmp, "app", "views", "admin", "post", "edit.html"),
        filepath.Join(tmp, "app", "views", "admin", "layouts", "admin.html"),
        filepath.Join(tmp, "app", "assets", "admin", "admin.css"),
        filepath.Join(tmp, "app", "admin", "README.md"),
    }
    for _, p := range paths {
        if _, err := os.Stat(p); err != nil {
            t.Fatalf("expected generated file %s to exist: %v", p, err)
        }
    }
}
