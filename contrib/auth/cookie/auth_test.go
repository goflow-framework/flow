package cookie

import (
	"net/http"
	"net/http/httptest"
	"testing"

	flowpkg "github.com/undiegomejia/flow/pkg/flow"
)

type fakeUser struct {
	ID string
}

func TestMiddleware_AttachesUserFromSession(t *testing.T) {
	// Create a session manager with a deterministic secret.
	secret := []byte("test-secret-32-bytes-long------")
	sm := flowpkg.NewSessionManager(secret, "flow_session")

	// Build a properly encrypted cookie by running a real Set through the
	// session middleware.  This avoids replicating the (now-encrypted) internal
	// encoding logic and ensures the test stays valid across format changes.
	cookieValue := func() string {
		setReq := httptest.NewRequest("GET", "/set", nil)
		setRec := httptest.NewRecorder()

		// Run the session middleware so a Session is wired into context.
		sm.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s := flowpkg.FromContext(r.Context())
			if s == nil {
				t.Fatal("no session in context during setup")
			}
			if err := s.Set("uid", "42"); err != nil {
				t.Fatalf("session Set: %v", err)
			}
		})).ServeHTTP(setRec, setReq)

		// Extract the cookie value that was written to the response.
		cookies := setRec.Result().Cookies()
		for _, c := range cookies {
			if c.Name == sm.CookieName {
				return c.Value
			}
		}
		t.Fatal("session cookie not set by Set()")
		return ""
	}()

	// simple lookup that maps the session uid to a fakeUser
	lookup := func(id interface{}) (interface{}, error) {
		if s, ok := id.(string); ok && s == "42" {
			return &fakeUser{ID: s}, nil
		}
		return nil, nil
	}

	mw := Middleware(sm, "uid", lookup)

	// Build the actual test request carrying the encrypted cookie.
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: sm.CookieName, Value: cookieValue})

	rr := httptest.NewRecorder()

	// Session middleware must run before auth middleware so the session is
	// decoded into context before Middleware tries to read from it.
	sessionMw := sm.Middleware()
	h := sessionMw(mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := FromContext(r.Context())
		if !ok {
			t.Fatalf("expected user in context")
		}
		fu, ok := u.(*fakeUser)
		if !ok {
			t.Fatalf("unexpected user type: %#v", u)
		}
		if fu.ID != "42" {
			t.Fatalf("unexpected user id: %s", fu.ID)
		}
		w.WriteHeader(http.StatusOK)
	})))

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rr.Code)
	}
}
