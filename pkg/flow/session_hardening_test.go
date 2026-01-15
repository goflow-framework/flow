package flow

import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

func TestSessionCookieHardening_AppendsSecureAndSameSite(t *testing.T) {
    h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        http.SetCookie(w, &http.Cookie{Name: "s", Value: "v", Path: "/"})
        w.WriteHeader(200)
    })

    hw := SessionCookieHardening()(h)
    req := httptest.NewRequest("GET", "https://example.local/", nil)
    rr := httptest.NewRecorder()
    hw.ServeHTTP(rr, req)

    // examine Set-Cookie header
    sc := rr.Header().Get("Set-Cookie")
    if sc == "" {
        t.Fatalf("expected Set-Cookie header, got empty")
    }
    if !strings.Contains(sc, "Secure") {
        t.Fatalf("expected Secure attribute added, got %q", sc)
    }
    if !strings.Contains(strings.ToLower(sc), "samesite=lax") {
        t.Fatalf("expected SameSite=Lax attribute added, got %q", sc)
    }
}
