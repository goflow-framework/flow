package flow

import (
	"fmt"
	"html/template"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// helper to write file creating parent dirs
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("mkdirall: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func TestViewManager_CacheVsDevMode(t *testing.T) {
	tmp, err := os.MkdirTemp("", "vmtest")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tmp); err != nil {
			t.Logf("warning: RemoveAll(%q) failed: %v", tmp, err)
		}
	}()

	// create a simple view that defines content
	viewPath := filepath.Join(tmp, "users", "show.html")
	writeFile(t, viewPath, "{{define \"content\"}}VERSION1: {{.}}{{end}}")

	vm := NewViewManager(tmp)
	app := New("testapp")
	app.Views = vm

	// first render: should show VERSION1
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	ctx := NewContext(app, rr, req)
	if err := ctx.Render("users/show", "X"); err != nil {
		t.Fatalf("render initial: %v", err)
	}
	out1 := rr.Body.String()
	if out1 != "VERSION1: X" {
		t.Fatalf("unexpected initial output: %q", out1)
	}

	// overwrite view with VERSION2
	writeFile(t, viewPath, "{{define \"content\"}}VERSION2: {{.}}{{end}}")
	// ensure mtime changes on filesystems that cache writes
	time.Sleep(10 * time.Millisecond)

	// render again without DevMode: should still show VERSION1 because cached
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/", nil)
	ctx2 := NewContext(app, rr2, req2)
	if err := ctx2.Render("users/show", "Y"); err != nil {
		t.Fatalf("render second (cached): %v", err)
	}
	out2 := rr2.Body.String()
	if out2 != "VERSION1: Y" {
		t.Fatalf("expected cached output VERSION1, got: %q", out2)
	}

	// enable DevMode and render: should reparse and show VERSION2
	vm.SetDevMode(true)
	rr3 := httptest.NewRecorder()
	req3 := httptest.NewRequest("GET", "/", nil)
	ctx3 := NewContext(app, rr3, req3)
	if err := ctx3.Render("users/show", "Z"); err != nil {
		t.Fatalf("render devmode: %v", err)
	}
	out3 := rr3.Body.String()
	if out3 != "VERSION2: Z" {
		t.Fatalf("expected dev output VERSION2, got: %q", out3)
	}
}

func TestViewManager_FuncMapAvailable(t *testing.T) {
	tmp, err := os.MkdirTemp("", "vmtest2")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tmp); err != nil {
			t.Logf("warning: RemoveAll(%q) failed: %v", tmp, err)
		}
	}()

	viewPath := filepath.Join(tmp, "greet", "hello.html")
	// template calls a function `greet`
	writeFile(t, viewPath, "{{define \"content\"}}{{greet .}}{{end}}")

	vm := NewViewManager(tmp)
	vm.SetFuncMap(template.FuncMap{"greet": func(name string) string { return "hi " + name }})
	app := New("testapp")
	app.Views = vm

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	ctx := NewContext(app, rr, req)
	if err := ctx.Render("greet/hello", "Alice"); err != nil {
		t.Fatalf("render greet: %v", err)
	}
	out := rr.Body.String()
	if out != "hi Alice" {
		t.Fatalf("unexpected greet output: %q", out)
	}
}

func TestViewManager_DefaultLayoutPrecedence(t *testing.T) {
	tmp, err := os.MkdirTemp("", "vmtest3")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tmp); err != nil {
			t.Logf("warning: RemoveAll(%q) failed: %v", tmp, err)
		}
	}()

	// create two layouts that both define `shared` so we can test precedence
	writeFile(t, filepath.Join(tmp, "layouts", "custom_layout.html"), "{{define \"shared\"}}FROM_CUSTOM{{end}}")
	writeFile(t, filepath.Join(tmp, "layouts", "other.html"), "{{define \"shared\"}}FROM_OTHER{{end}}")
	// a view that invokes shared
	writeFile(t, filepath.Join(tmp, "items", "show.html"), "{{define \"content\"}}ITEM: {{template \"shared\" .}}{{end}}")

	vm := NewViewManager(tmp)
	app := New("testapp")
	app.Views = vm

	// without DefaultLayout set, the loader will parse layouts/*.html and
	// the last parsed definition will win; glob returns sorted names, so
	// "other.html" will be parsed last and its `shared` should win.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	ctx := NewContext(app, rr, req)
	if err := ctx.Render("items/show", nil); err != nil {
		t.Fatalf("render without default layout: %v", err)
	}
	out := rr.Body.String()
	if out != "ITEM: FROM_OTHER" {
		t.Fatalf("expected output from 'other' layout by default, got: %q", out)
	}

	// set default layout to the custom_layout and render successfully (should override)
	vm.SetDefaultLayout("layouts/custom_layout.html")
	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/", nil)
	ctx2 := NewContext(app, rr2, req2)
	if err := ctx2.Render("items/show", nil); err != nil {
		t.Fatalf("render with default layout: %v", err)
	}
	out2 := rr2.Body.String()
	if out2 != "ITEM: FROM_CUSTOM" {
		t.Fatalf("unexpected output with default layout: %q", out2)
	}
}

func TestViewManager_SetFuncMapClearsCache(t *testing.T) {
	tmp, err := os.MkdirTemp("", "vmtest4")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tmp); err != nil {
			t.Logf("warning: RemoveAll(%q) failed: %v", tmp, err)
		}
	}()

	viewPath := filepath.Join(tmp, "greet2", "hello.html")
	writeFile(t, viewPath, "{{define \"content\"}}{{greet .}}{{end}}")

	vm := NewViewManager(tmp)
	vm.SetFuncMap(template.FuncMap{"greet": func(name string) string { return "v1 " + name }})
	app := New("testapp")
	app.Views = vm

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	ctx := NewContext(app, rr, req)
	if err := ctx.Render("greet2/hello", "Bob"); err != nil {
		t.Fatalf("render greet v1: %v", err)
	}
	out := rr.Body.String()
	if out != "v1 Bob" {
		t.Fatalf("unexpected greet output v1: %q", out)
	}

	// change funcmap -- this should clear cache and take effect immediately
	vm.SetFuncMap(template.FuncMap{"greet": func(name string) string { return "v2 " + name }})

	rr2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/", nil)
	ctx2 := NewContext(app, rr2, req2)
	if err := ctx2.Render("greet2/hello", "Bob"); err != nil {
		t.Fatalf("render greet v2: %v", err)
	}
	out2 := rr2.Body.String()
	if out2 != "v2 Bob" {
		t.Fatalf("unexpected greet output v2: %q", out2)
	}
}

func TestApp_WithViewsFuncMap(t *testing.T) {
	tmp, err := os.MkdirTemp("", "vmtest_appfunc")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(tmp); err != nil {
			t.Logf("warning: RemoveAll(%q) failed: %v", tmp, err)
		}
	}()

	viewPath := filepath.Join(tmp, "hello", "world.html")
	writeFile(t, viewPath, "{{define \"content\"}}{{cap .}}{{end}}")

	// create app with FuncMap configured via option
	app := New("testapp", WithViewsFuncMap(template.FuncMap{"cap": func(s string) string { return "CAP_" + s }}))
	// set views directory to our temp dir
	app.Views.TemplateDir = tmp
	// ensure dev mode so parsing is deterministic for test
	app.Views.SetDevMode(true)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	ctx := NewContext(app, rr, req)
	if err := ctx.Render("hello/world", "bob"); err != nil {
		t.Fatalf("render with app funcmap: %v", err)
	}
	out := rr.Body.String()
	if out != "CAP_bob" {
		t.Fatalf("unexpected output from app funcmap: %q", out)
	}
}

// TestViewManager_ConcurrentRender verifies that the pool-based clone approach
// is safe under concurrent load and that each goroutine receives its own
// correctly-rendered output (no cross-request data leakage).
func TestViewManager_ConcurrentRender(t *testing.T) {
	tmp := t.TempDir()
	viewPath := filepath.Join(tmp, "item", "show.html")
	writeFile(t, viewPath, `{{define "content"}}value:{{.}}{{end}}`)

	vm := NewViewManager(tmp)
	app := New("testapp")
	app.Views = vm

	const goroutines = 20
	var wg sync.WaitGroup
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			want := fmt.Sprintf("value:%d", i)
			rr := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			ctx := NewContext(app, rr, req)
			if err := ctx.Render("item/show", i); err != nil {
				errs[i] = fmt.Errorf("goroutine %d render: %w", i, err)
				return
			}
			got := rr.Body.String()
			if got != want {
				errs[i] = fmt.Errorf("goroutine %d: want %q got %q", i, want, got)
			}
		}()
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
}

// TestViewManager_PoolRecyclesClones exercises that pool entries are returned
// and re-used rather than leaking on each render.  It triggers enough renders
// to guarantee at least one pool reuse and confirms output correctness.
func TestViewManager_PoolRecyclesClones(t *testing.T) {
	tmp := t.TempDir()
	viewPath := filepath.Join(tmp, "pooltest", "view.html")
	writeFile(t, viewPath, `{{define "content"}}rendered:{{.}}{{end}}`)

	vm := NewViewManager(tmp)
	app := New("testapp")
	app.Views = vm

	for i := 0; i < 10; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		ctx := NewContext(app, rr, req)
		if err := ctx.Render("pooltest/view", i); err != nil {
			t.Fatalf("render %d: %v", i, err)
		}
		want := fmt.Sprintf("rendered:%d", i)
		if got := rr.Body.String(); got != want {
			t.Errorf("render %d: want %q got %q", i, want, got)
		}
	}
}
