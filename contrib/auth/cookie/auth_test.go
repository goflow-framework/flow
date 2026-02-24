package cookie

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	flowpkg "github.com/undiegomejia/flow/pkg/flow"
)

type fakeUser struct {
	ID string
}

func TestMiddleware_AttachesUserFromSession(t *testing.T) {
	// create a deterministic session manager so we can craft a cookie
	secret := []byte("test-secret-32-bytes-long------")
	sm := flowpkg.NewSessionManager(secret, "flow_session")

	// craft a session map with user id
	vals := map[string]interface{}{"uid": "42"}
	// replicate SessionManager.encodeForCookie logic here (unexported in package)
	b, err := json.Marshal(vals)
	if err != nil {
		t.Fatalf("marshal session: %v", err)
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(b)
	sig := mac.Sum(nil)
	enc := base64.RawURLEncoding.EncodeToString(b) + "|" + hex.EncodeToString(sig)

	// simple lookup that maps the session uid to a fakeUser
	lookup := func(id interface{}) (interface{}, error) {
		if s, ok := id.(string); ok && s == "42" {
			return &fakeUser{ID: s}, nil
		}
		return nil, nil
	}

	mw := Middleware(sm, "uid", lookup)

	// build request with cookie
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: sm.CookieName, Value: enc})

	rr := httptest.NewRecorder()

	// ensure session middleware runs before our auth middleware so the
	// session is decoded into context.
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
