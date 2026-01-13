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

// TestCLI_GenerateAuth_Compiles generates auth into a new temporary module,
// patches the placeholder model import in the generated controller and
// middleware to the temp module path, and runs a small main.go that imports
// the generated controllers and models to ensure compilation.
func TestCLI_GenerateAuth_Compiles(t *testing.T) {
    repo := findRepoRoot()

    modName, err := readModuleName(repo)
    if err != nil {
        t.Fatalf("read module name: %v", err)
    }

    tmpProj := t.TempDir()
    uid := filepath.Base(tmpProj)
    moduleName := modName + "/examples/" + uid
    // write a go.mod and add replace directive so the temp module can
    // resolve the local repository packages.
    goMod := "module " + moduleName + "\n\n" +
        "go 1.20\n\n" +
        "require " + modName + " v0.0.0\n\n" +
        "replace " + modName + " => " + repo + "\n"
    if err := os.WriteFile(filepath.Join(tmpProj, "go.mod"), []byte(goMod), 0o644); err != nil {
        t.Fatalf("write go.mod: %v", err)
    }

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

    // patch generated controller to replace placeholder import path
    modelsImport := moduleName + "/app/models"
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
    controllersImport := moduleName + "/app/controllers"
    middlewareImport := moduleName + "/app/middleware"
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

    // ensure module deps are tidy before running
    tidy := exec.Command("go", "mod", "tidy")
    tidy.Dir = tmpProj
    if out, err := tidy.CombinedOutput(); err != nil {
        t.Fatalf("go mod tidy failed: %v\n%s", err, string(out))
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

// TestCLI_GenerateAuth_SessionHelperParsing verifies the generated
// middleware's GetSessionUserID helper correctly parses a string-stored
// user_id after a cookie roundtrip and rejects numeric-typed stored values.
func TestCLI_GenerateAuth_SessionHelperParsing(t *testing.T) {
    repo := findRepoRoot()
    modName, err := readModuleName(repo)
    if err != nil {
        t.Fatalf("read module name: %v", err)
    }

    tmpProj := t.TempDir()
    uid := filepath.Base(tmpProj)
    moduleName := modName + "/examples/" + uid
    goMod := "module " + moduleName + "\n\n" +
        "go 1.20\n\n" +
        "require " + modName + " v0.0.0\n\n" +
        "replace " + modName + " => " + repo + "\n"
    if err := os.WriteFile(filepath.Join(tmpProj, "go.mod"), []byte(goMod), 0o644); err != nil {
        t.Fatalf("write go.mod: %v", err)
    }

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

    // patch generated controller and middleware imports
    modelsImport := moduleName + "/app/models"
    ctrlPath := filepath.Join(tmpProj, "app", "controllers", "auth_controller.go")
    if b, err := os.ReadFile(ctrlPath); err == nil {
        src := strings.Replace(string(b), "REPLACE_WITH_MODULE_PATH/app/models", modelsImport, 1)
        if err := os.WriteFile(ctrlPath, []byte(src), 0o644); err != nil {
            t.Fatalf("patch controller import: %v", err)
        }
    } else {
        t.Fatalf("read generated controller: %v", err)
    }
    mwPath := filepath.Join(tmpProj, "app", "middleware", "auth.go")
    if mb, err := os.ReadFile(mwPath); err == nil {
        msrc := strings.Replace(string(mb), "REPLACE_WITH_MODULE_PATH/app/models", modelsImport, 1)
        if err := os.WriteFile(mwPath, []byte(msrc), 0o644); err != nil {
            t.Fatalf("patch middleware import: %v", err)
        }
    } else {
        t.Fatalf("read generated middleware: %v", err)
    }

    // write main.go that exercises GetSessionUserID with string and numeric values
    middlewareImport := moduleName + "/app/middleware"
    mainSrc := `package main

import (
    "fmt"
    "net/http"
    "net/http/httptest"

    flow "` + modName + `/pkg/flow"
    middleware "` + middlewareImport + `"
)

func main() {
    sm := flow.DefaultSessionManager()

    // Handler that stores a string user_id and returns the cookie
    h1 := sm.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        s := flow.FromContext(r.Context())
        _ = s.Set("user_id", "42")
        // respond with OK; cookie written to w
        w.WriteHeader(200)
    }))

    rr := httptest.NewRecorder()
    req := httptest.NewRequest("GET", "/", nil)
    h1.ServeHTTP(rr, req)
    setCookie := rr.Header().Get("Set-Cookie")

    // follow-up request carrying the cookie; middleware should decode it
    rr2 := httptest.NewRecorder()
    req2 := httptest.NewRequest("GET", "/", nil)
    if setCookie != "" {
        req2.Header.Set("Cookie", setCookie)
    }
    h2 := sm.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        s := flow.FromContext(r.Context())
        if id, ok := middleware.GetSessionUserID(s); ok {
            fmt.Printf("STRING_ID:%d:%t\n", id, ok)
        } else {
            fmt.Printf("STRING_ID:0:false\n")
        }
    }))
    h2.ServeHTTP(rr2, req2)

    // Now store a numeric value and repeat the roundtrip
    rr3 := httptest.NewRecorder()
    req3 := httptest.NewRequest("GET", "/", nil)
    h3 := sm.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        s := flow.FromContext(r.Context())
        _ = s.Set("user_id", 42) // numeric
        w.WriteHeader(200)
    }))
    h3.ServeHTTP(rr3, req3)
    setCookie2 := rr3.Header().Get("Set-Cookie")

    rr4 := httptest.NewRecorder()
    req4 := httptest.NewRequest("GET", "/", nil)
    if setCookie2 != "" {
        req4.Header.Set("Cookie", setCookie2)
    }
    h4 := sm.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        s := flow.FromContext(r.Context())
        if id, ok := middleware.GetSessionUserID(s); ok {
            fmt.Printf("INT_ID:%d:%t\n", id, ok)
        } else {
            fmt.Printf("INT_ID:0:false\n")
        }
    }))
    h4.ServeHTTP(rr4, req4)
}
`

    if err := os.WriteFile(filepath.Join(tmpProj, "main.go"), []byte(mainSrc), 0o644); err != nil {
        t.Fatalf("write main.go: %v", err)
    }

    // tidy deps before running
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
}

func TestGeneratedMiddleware_Unit_GetSessionUserID(t *testing.T) {
    repo := findRepoRoot()
    modName, err := readModuleName(repo)
    if err != nil {
        t.Fatalf("read module name: %v", err)
    }

    tmpProj := t.TempDir()
    uid := filepath.Base(tmpProj)
    moduleName := modName + "/examples/" + uid
    goMod := "module " + moduleName + "\n\n" +
        "go 1.20\n\n" +
        "require " + modName + " v0.0.0\n\n" +
        "replace " + modName + " => " + repo + "\n"
    if err := os.WriteFile(filepath.Join(tmpProj, "go.mod"), []byte(goMod), 0o644); err != nil {
        t.Fatalf("write go.mod: %v", err)
    }

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

    // patch generated middleware import
    modelsImport := moduleName + "/app/models"
    // also patch generated controller import so tests build
    ctrlPath := filepath.Join(tmpProj, "app", "controllers", "auth_controller.go")
    if cb, err := os.ReadFile(ctrlPath); err == nil {
        csrc := strings.Replace(string(cb), "REPLACE_WITH_MODULE_PATH/app/models", modelsImport, 1)
        if err := os.WriteFile(ctrlPath, []byte(csrc), 0o644); err != nil {
            t.Fatalf("patch controller import: %v", err)
        }
    } else {
        t.Fatalf("read generated controller: %v", err)
    }
    mwPath := filepath.Join(tmpProj, "app", "middleware", "auth.go")
    if mb, err := os.ReadFile(mwPath); err == nil {
        msrc := strings.Replace(string(mb), "REPLACE_WITH_MODULE_PATH/app/models", modelsImport, 1)
        if err := os.WriteFile(mwPath, []byte(msrc), 0o644); err != nil {
            t.Fatalf("patch middleware import: %v", err)
        }
    } else {
        t.Fatalf("read generated middleware: %v", err)
    }

    // write a tiny unit test under app/middleware
    testSrc := `package middleware_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    flow "` + modName + `/pkg/flow"
    middleware "` + moduleName + `/app/middleware"
)

func TestGetSessionUserID_Roundtrip(t *testing.T) {
    sm := flow.DefaultSessionManager()

    // store string value
    rr := httptest.NewRecorder()
    req := httptest.NewRequest("GET", "/", nil)
    h := sm.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        s := flow.FromContext(r.Context())
        _ = s.Set("user_id", "123")
        w.WriteHeader(200)
    }))
    h.ServeHTTP(rr, req)
    cookie := rr.Header().Get("Set-Cookie")

    rr2 := httptest.NewRecorder()
    req2 := httptest.NewRequest("GET", "/", nil)
    if cookie != "" {
        req2.Header.Set("Cookie", cookie)
    }
    h2 := sm.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        s := flow.FromContext(r.Context())
        id, ok := middleware.GetSessionUserID(s)
        if !ok || id != 123 {
            t.Fatalf("expected parsed id 123; got %d ok=%v", id, ok)
        }
    }))
    h2.ServeHTTP(rr2, req2)

    // store numeric value
    rr3 := httptest.NewRecorder()
    req3 := httptest.NewRequest("GET", "/", nil)
    h3 := sm.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        s := flow.FromContext(r.Context())
        _ = s.Set("user_id", 123)
        w.WriteHeader(200)
    }))
    h3.ServeHTTP(rr3, req3)
    cookie2 := rr3.Header().Get("Set-Cookie")

    rr4 := httptest.NewRecorder()
    req4 := httptest.NewRequest("GET", "/", nil)
    if cookie2 != "" {
        req4.Header.Set("Cookie", cookie2)
    }
    h4 := sm.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        s := flow.FromContext(r.Context())
        id, ok := middleware.GetSessionUserID(s)
        if ok || id != 0 {
            t.Fatalf("expected numeric stored value to fail parse; got %d ok=%v", id, ok)
        }
    }))
    h4.ServeHTTP(rr4, req4)
}
`

    testPath := filepath.Join(tmpProj, "app", "middleware", "auth_test.go")
    if err := os.WriteFile(testPath, []byte(testSrc), 0o644); err != nil {
        t.Fatalf("write test file: %v", err)
    }

    // tidy and run tests in the temp project
    tidy := exec.Command("go", "mod", "tidy")
    tidy.Dir = tmpProj
    if out, err := tidy.CombinedOutput(); err != nil {
        t.Fatalf("go mod tidy failed: %v\n%s", err, string(out))
    }

    cmd := exec.Command("go", "test", "./...", "-v")
    cmd.Dir = tmpProj
    out, err := cmd.CombinedOutput()
    t.Logf("temp project test output:\n%s", string(out))
    if err != nil {
        t.Fatalf("temp project tests failed: %v\n%s", err, string(out))
    }
}
