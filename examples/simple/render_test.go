package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	controllers "github.com/undiegomejia/flow/examples/simple/app/controllers"
	flow "github.com/undiegomejia/flow/pkg/flow"
)

func TestExampleRenderShow(t *testing.T) {
	app := flow.New("examples-test")
	// when running tests in this package the working directory is the
	// package directory, so use the relative path to the views folder.
	app.Views = flow.NewViewManager("app/views")

	r := flow.NewRouter(app)
	users := controllers.NewUsersController(app)
	if err := r.Resources("users", users); err != nil {
		t.Fatalf("register resources: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/users/7", nil)
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	body, _ := io.ReadAll(rr.Body)
	if !contains(string(body), "ID: 7") {
		t.Fatalf("expected rendered body to include ID: 7, got %q", string(body))
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(s) > len(sub) && (indexOf(s, sub) >= 0)))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
