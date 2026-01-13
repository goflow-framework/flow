package i18n

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDirAndGet(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "en.yaml")
	content := `admin:
  posts:
    index: "Posts"
    new: "New Post"
auth:
  login: "Login"
`
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatalf("write en.yaml: %v", err)
	}
	mgr := NewManager()
	if err := mgr.LoadDir(tmp); err != nil {
		t.Fatalf("LoadDir failed: %v", err)
	}
	if v, ok := mgr.Get("en", "admin.posts.index"); !ok || v != "Posts" {
		t.Fatalf("expected admin.posts.index=Posts; got %q ok=%v", v, ok)
	}
	if v, ok := mgr.Get("en", "auth.login"); !ok || v != "Login" {
		t.Fatalf("expected auth.login=Login; got %q ok=%v", v, ok)
	}
}
