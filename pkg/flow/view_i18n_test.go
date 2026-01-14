package flow

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/undiegomejia/flow/pkg/i18n"
)

func TestTemplateFuncT_RendersTranslation(t *testing.T) {
	tmp := t.TempDir()
	viewsDir := filepath.Join(tmp, "views")
	if err := os.MkdirAll(viewsDir, 0o755); err != nil {
		t.Fatalf("mkdir views: %v", err)
	}
	// write a simple template that uses the T function
	tmplPath := filepath.Join(viewsDir, "testtmpl.html")
	if err := os.WriteFile(tmplPath, []byte(`{{ T "admin.posts.index" }}`), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	// write i18n en.yaml
	i18nDir := filepath.Join(tmp, "app", "i18n")
	if err := os.MkdirAll(i18nDir, 0o755); err != nil {
		t.Fatalf("mkdir i18n: %v", err)
	}
	enPath := filepath.Join(i18nDir, "en.yaml")
	enContent := `admin:
  posts:
    index: "Posts"
`
	if err := os.WriteFile(enPath, []byte(enContent), 0o644); err != nil {
		t.Fatalf("write en.yaml: %v", err)
	}

	// prepare i18n manager
	mgr := i18n.NewManager()
	if err := mgr.LoadDir(i18nDir); err != nil {
		t.Fatalf("load i18n: %v", err)
	}

	// prepare app with view manager
	app := New("test-app")
	app.Views = NewViewManager(viewsDir)

	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := NewContext(app, w, r)
		defer PutContext(ctx)
		if err := app.Views.Render("testtmpl", nil, ctx); err != nil {
			http.Error(w, err.Error(), 500)
		}
	})

	// wrap with i18n middleware
	handler = i18n.Middleware(mgr, "en")(handler)

	srv := httptest.NewServer(handler)
	defer srv.Close()

	res, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	s := string(b)
	if s != "Posts" {
		t.Fatalf("unexpected body: %q", s)
	}
}
