package flow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ── /healthz (readiness) ──────────────────────────────────────────────────────

// TestHealthz_RunningReturns200 verifies /healthz returns 200 while running.
func TestHealthz_RunningReturns200(t *testing.T) {
	a := New("hztest", WithHealthz())
	a.Addr = ":0"
	if err := a.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = a.Shutdown(ctx)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, HealthzPath, nil)
	a.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", rec.Code, rec.Body.String())
	}
	assertHealthContentType(t, rec)
	assertHealthJSONStatus(t, rec, "ok")
}

// TestHealthz_IdleReturns503 verifies /healthz returns 503 before Start.
func TestHealthz_IdleReturns503(t *testing.T) {
	a := New("hztest-idle", WithHealthz())
	// Do NOT call a.Start() — state stays 0.

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, HealthzPath, nil)
	a.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when idle, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertHealthJSONStatus(t, rec, "unavailable")
}

// TestHealthz_AfterShutdownReturns503 verifies /healthz returns 503 after shutdown.
func TestHealthz_AfterShutdownReturns503(t *testing.T) {
	a := New("hztest-shutdown", WithHealthz())
	a.Addr = ":0"
	if err := a.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := a.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, HealthzPath, nil)
	a.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 after shutdown, got %d", rec.Code)
	}
}

// ── /livez (liveness) ─────────────────────────────────────────────────────────

// TestLivez_IdleReturns200 verifies /livez always returns 200 even before Start.
func TestLivez_IdleReturns200(t *testing.T) {
	a := New("livez-idle", WithHealthz())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, LivezPath, nil)
	a.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /livez (idle), got %d", rec.Code)
	}
	assertHealthJSONStatus(t, rec, "ok")
}

// TestLivez_RunningReturns200 verifies /livez returns 200 while running.
func TestLivez_RunningReturns200(t *testing.T) {
	a := New("livez-running", WithHealthz())
	a.Addr = ":0"
	if err := a.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = a.Shutdown(ctx)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, LivezPath, nil)
	a.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /livez (running), got %d", rec.Code)
	}
	assertHealthJSONStatus(t, rec, "ok")
}

// ── method enforcement ────────────────────────────────────────────────────────

// TestHealthz_MethodNotAllowed verifies POST gets 405 on both endpoints.
func TestHealthz_MethodNotAllowed(t *testing.T) {
	for _, path := range []string{HealthzPath, LivezPath} {
		path := path
		t.Run(path, func(t *testing.T) {
			a := New("hz-method", WithHealthz())
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, path, nil)
			a.ServeHTTP(rec, req)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s: expected 405, got %d", path, rec.Code)
			}
		})
	}
}

// TestHealthz_HeadMethod verifies HEAD /healthz returns 200 while running.
func TestHealthz_HeadMethod(t *testing.T) {
	a := New("hz-head", WithHealthz())
	a.Addr = ":0"
	if err := a.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = a.Shutdown(ctx)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodHead, HealthzPath, nil)
	a.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("HEAD /healthz: expected 200, got %d", rec.Code)
	}
}

// ── idempotency ───────────────────────────────────────────────────────────────

// TestEnableHealthz_Idempotent verifies that calling EnableHealthz twice does
// not panic.
func TestEnableHealthz_Idempotent(t *testing.T) {
	a := New("hz-idempotent")
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("second EnableHealthz panicked: %v", r)
		}
	}()
	a.EnableHealthz()
	a.EnableHealthz() // must be a no-op
}

// ── response body ─────────────────────────────────────────────────────────────

// TestWithHealthz_AppNameAndUptimeInResponse verifies the JSON body contains
// the App name and an uptime field while running.
func TestWithHealthz_AppNameAndUptimeInResponse(t *testing.T) {
	a := New("my-service", WithHealthz())
	a.Addr = ":0"
	if err := a.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = a.Shutdown(ctx)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, HealthzPath, nil)
	a.ServeHTTP(rec, req)

	var resp healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if resp.App != "my-service" {
		t.Fatalf("expected app=my-service, got %q", resp.App)
	}
	if resp.Uptime == "" {
		t.Fatal("expected non-empty uptime in response")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func assertHealthContentType(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	ct := rec.Header().Get("Content-Type")
	want := "application/json"
	if len(ct) < len(want) || ct[:len(want)] != want {
		t.Errorf("Content-Type: got %q, want prefix %q", ct, want)
	}
}

func assertHealthJSONStatus(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	body := rec.Body.Bytes()
	var resp healthResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode JSON: %v — body: %s", err, body)
	}
	if resp.Status != want {
		t.Errorf("status: got %q, want %q", resp.Status, want)
	}
}
