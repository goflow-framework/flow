package flow

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func okRateHandler(t *testing.T, called *int32) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(called, 1)
		w.WriteHeader(http.StatusOK)
	})
}

func makeReq(remoteAddr, xff string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	return req
}

// ---------------------------------------------------------------------------
// RateLimitMiddleware — basic allow/reject behaviour
// ---------------------------------------------------------------------------

func TestRateLimit_AllowsRequests(t *testing.T) {
	t.Parallel()

	var called int32
	mw := RateLimitMiddleware(100, 100)(okRateHandler(t, &called))

	for i := 0; i < 10; i++ {
		req := makeReq("1.2.3.4:9000", "")
		rr := httptest.NewRecorder()
		mw.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 on attempt %d, got %d", i, rr.Code)
		}
	}
	if atomic.LoadInt32(&called) != 10 {
		t.Fatalf("expected handler called 10 times, got %d", called)
	}
}

func TestRateLimit_RejectsWhenExceeded(t *testing.T) {
	t.Parallel()

	var called int32
	// burst=1, rps=1 → only the first request in a burst passes
	mw := RateLimitMiddleware(1, 1)(okRateHandler(t, &called))

	var success, rejected int
	for i := 0; i < 5; i++ {
		rr := httptest.NewRecorder()
		mw.ServeHTTP(rr, makeReq("2.2.2.2:0", ""))
		switch rr.Code {
		case http.StatusOK:
			success++
		case http.StatusTooManyRequests:
			rejected++
		default:
			t.Fatalf("unexpected status %d", rr.Code)
		}
	}
	if rejected == 0 {
		t.Fatal("expected at least one 429, got zero")
	}
	if atomic.LoadInt32(&called) != int32(success) {
		t.Fatalf("handler call count mismatch: success=%d called=%d", success, called)
	}
}

func TestRateLimit_ZeroRPSIsNoOp(t *testing.T) {
	t.Parallel()

	var called int32
	mw := RateLimitMiddleware(0, 0)(okRateHandler(t, &called))

	for i := 0; i < 20; i++ {
		rr := httptest.NewRecorder()
		mw.ServeHTTP(rr, makeReq("3.3.3.3:0", ""))
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 with rps=0 (no-op), got %d", rr.Code)
		}
	}
}

func TestRateLimit_RetryAfterHeader(t *testing.T) {
	t.Parallel()

	mw := RateLimitMiddleware(1, 1)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Drain the burst, then confirm Retry-After is set on 429.
	var last *httptest.ResponseRecorder
	for i := 0; i < 5; i++ {
		rr := httptest.NewRecorder()
		mw.ServeHTTP(rr, makeReq("4.4.4.4:0", ""))
		last = rr
	}
	if last.Code == http.StatusTooManyRequests {
		if last.Header().Get("Retry-After") == "" {
			t.Fatal("expected Retry-After header on 429 response")
		}
	}
}

// ---------------------------------------------------------------------------
// clientIPFromRequest — security-critical path
// ---------------------------------------------------------------------------

func TestClientIPFromRequest_NoProxies_IgnoresXFF(t *testing.T) {
	t.Parallel()

	req := makeReq("1.2.3.4:9999", "9.9.9.9")
	// No trusted proxies → XFF must be ignored, RemoteAddr wins.
	got := clientIPFromRequest(req, nil)
	if got != "1.2.3.4" {
		t.Fatalf("expected RemoteAddr IP 1.2.3.4, got %q", got)
	}
}

func TestClientIPFromRequest_NoProxies_EmptyXFF(t *testing.T) {
	t.Parallel()

	req := makeReq("5.6.7.8:1234", "")
	got := clientIPFromRequest(req, nil)
	if got != "5.6.7.8" {
		t.Fatalf("expected 5.6.7.8, got %q", got)
	}
}

func TestClientIPFromRequest_UntrustedPeer_IgnoresXFF(t *testing.T) {
	t.Parallel()

	proxies := MustParseCIDRs([]string{"10.0.0.0/8"})
	// RemoteAddr is 1.2.3.4 — NOT in 10.0.0.0/8 → XFF ignored.
	req := makeReq("1.2.3.4:9999", "9.9.9.9")
	got := clientIPFromRequest(req, proxies)
	if got != "1.2.3.4" {
		t.Fatalf("untrusted peer: expected RemoteAddr 1.2.3.4, got %q", got)
	}
}

func TestClientIPFromRequest_TrustedPeer_ExtractsRealClient(t *testing.T) {
	t.Parallel()

	proxies := MustParseCIDRs([]string{"10.0.0.0/8"})
	// RemoteAddr is 10.0.0.1 (trusted proxy).
	// XFF = "203.0.113.5, 10.0.0.2" → rightmost trusted proxy stripped,
	// leftmost non-trusted entry "203.0.113.5" is the real client.
	req := makeReq("10.0.0.1:80", "203.0.113.5, 10.0.0.2")
	got := clientIPFromRequest(req, proxies)
	if got != "203.0.113.5" {
		t.Fatalf("expected real client 203.0.113.5, got %q", got)
	}
}

func TestClientIPFromRequest_SingleTrustedProxy_SingleHop(t *testing.T) {
	t.Parallel()

	proxies := MustParseCIDRs([]string{"192.168.1.0/24"})
	// Proxy is 192.168.1.1; client is 8.8.8.8.
	req := makeReq("192.168.1.1:443", "8.8.8.8")
	got := clientIPFromRequest(req, proxies)
	if got != "8.8.8.8" {
		t.Fatalf("expected 8.8.8.8, got %q", got)
	}
}

func TestClientIPFromRequest_AllXFFTrusted_FallsBackToRemoteAddr(t *testing.T) {
	t.Parallel()

	proxies := MustParseCIDRs([]string{"10.0.0.0/8"})
	// All XFF entries are trusted proxies — fall back to RemoteAddr.
	req := makeReq("10.0.0.1:80", "10.0.0.2, 10.0.0.3")
	got := clientIPFromRequest(req, proxies)
	if got != "10.0.0.1" {
		t.Fatalf("expected RemoteAddr fallback 10.0.0.1, got %q", got)
	}
}

func TestClientIPFromRequest_MalformedXFFEntry_StopsParsing(t *testing.T) {
	t.Parallel()

	proxies := MustParseCIDRs([]string{"10.0.0.0/8"})
	// XFF contains a malformed entry — should stop and fall back to RemoteAddr.
	// Walk right-to-left: 10.0.0.2 (trusted) → INVALID (unparseable) → stop.
	req := makeReq("10.0.0.1:80", "203.0.113.5, INVALID, 10.0.0.2")
	got := clientIPFromRequest(req, proxies)
	if got != "10.0.0.1" {
		t.Fatalf("malformed XFF: expected RemoteAddr fallback 10.0.0.1, got %q", got)
	}
}

func TestClientIPFromRequest_RemoteAddrNoPort(t *testing.T) {
	t.Parallel()

	req := makeReq("5.5.5.5", "") // no port
	got := clientIPFromRequest(req, nil)
	if got != "5.5.5.5" {
		t.Fatalf("expected 5.5.5.5, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// MustParseCIDRs / ParseCIDRs
// ---------------------------------------------------------------------------

func TestMustParseCIDRs_Valid(t *testing.T) {
	t.Parallel()

	nets := MustParseCIDRs([]string{"10.0.0.0/8", "192.168.0.0/16"})
	if len(nets) != 2 {
		t.Fatalf("expected 2 networks, got %d", len(nets))
	}
}

func TestMustParseCIDRs_PanicsOnInvalid(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for invalid CIDR")
		}
	}()
	MustParseCIDRs([]string{"not-a-cidr"})
}

func TestParseCIDRs_ReturnsErrorOnInvalid(t *testing.T) {
	t.Parallel()

	_, err := ParseCIDRs([]string{"10.0.0.0/8", "bad"})
	if err == nil {
		t.Fatal("expected error for invalid CIDR")
	}
}

func TestParseCIDRs_Valid(t *testing.T) {
	t.Parallel()

	nets, err := ParseCIDRs([]string{"172.16.0.0/12"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nets) != 1 {
		t.Fatalf("expected 1 network, got %d", len(nets))
	}
}

// ---------------------------------------------------------------------------
// RateLimitMiddlewareWithOptions — per-instance isolation
// ---------------------------------------------------------------------------

func TestRateLimitMiddlewareWithOptions_PerClosureIsolation(t *testing.T) {
	t.Parallel()

	// Two middleware instances with separate maps should not share state.
	var c1, c2 int32
	mw1 := RateLimitMiddlewareWithOptions(RateLimitOptions{RPS: 1, Burst: 1})(okRateHandler(t, &c1))
	mw2 := RateLimitMiddlewareWithOptions(RateLimitOptions{RPS: 100, Burst: 100})(okRateHandler(t, &c2))

	// Drain mw1 for IP 6.6.6.6.
	for i := 0; i < 5; i++ {
		rr := httptest.NewRecorder()
		mw1.ServeHTTP(rr, makeReq("6.6.6.6:0", ""))
	}

	// mw2 should still allow the same IP freely.
	rr := httptest.NewRecorder()
	mw2.ServeHTTP(rr, makeReq("6.6.6.6:0", ""))
	if rr.Code != http.StatusOK {
		t.Fatalf("mw2 should be independent of mw1, got %d", rr.Code)
	}
}

func TestRateLimitMiddlewareWithOptions_TrustedProxyRateLimitsRealClient(t *testing.T) {
	t.Parallel()

	proxies := MustParseCIDRs([]string{"10.0.0.0/8"})

	// Rate-limit tightly (burst=1) so the second request from the same real
	// client IP should be rejected.
	mw := RateLimitMiddlewareWithOptions(RateLimitOptions{
		RPS:            1,
		Burst:          1,
		TrustedProxies: proxies,
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Same real client (7.7.7.7) through trusted proxy (10.0.0.1).
	req1 := makeReq("10.0.0.1:80", "7.7.7.7")
	req2 := makeReq("10.0.0.1:80", "7.7.7.7")

	rr1 := httptest.NewRecorder()
	mw.ServeHTTP(rr1, req1)

	rr2 := httptest.NewRecorder()
	mw.ServeHTTP(rr2, req2)

	if rr1.Code != http.StatusOK {
		t.Fatalf("first request should pass, got %d", rr1.Code)
	}
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request should be 429, got %d", rr2.Code)
	}
}

func TestRateLimitMiddlewareWithOptions_SpoofedXFFIgnoredWhenPeerUntrusted(t *testing.T) {
	t.Parallel()

	proxies := MustParseCIDRs([]string{"10.0.0.0/8"})

	var hits int32
	mw := RateLimitMiddlewareWithOptions(RateLimitOptions{
		RPS:            1,
		Burst:          1,
		TrustedProxies: proxies,
	})(okRateHandler(t, &hits))

	// Attacker at 5.5.5.5 (NOT a trusted proxy) spoofs XFF with different IPs
	// each time — both requests must be counted against 5.5.5.5.
	rr1 := httptest.NewRecorder()
	mw.ServeHTTP(rr1, makeReq("5.5.5.5:1234", "1.1.1.1"))
	rr2 := httptest.NewRecorder()
	mw.ServeHTTP(rr2, makeReq("5.5.5.5:1234", "2.2.2.2"))

	if rr1.Code != http.StatusOK {
		t.Fatalf("first request should pass, got %d", rr1.Code)
	}
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("spoofed XFF should not bypass rate limit: second request got %d", rr2.Code)
	}
}

// ---------------------------------------------------------------------------
// WithRateLimitOptions Option
// ---------------------------------------------------------------------------

func TestWithRateLimitOptions_WiresOnApp(t *testing.T) {
	t.Parallel()

	proxies := MustParseCIDRs([]string{"10.0.0.0/8"})
	app := New("test", WithRateLimitOptions(RateLimitOptions{
		RPS:            1,
		Burst:          1,
		TrustedProxies: proxies,
	}))

	var hits int32
	app.SetRouter(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))

	h := app.Handler()

	rr1 := httptest.NewRecorder()
	h.ServeHTTP(rr1, makeReq("10.0.0.1:80", "8.8.8.8"))

	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, makeReq("10.0.0.1:80", "8.8.8.8"))

	if rr1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rr1.Code)
	}
	if rr2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: expected 429, got %d", rr2.Code)
	}
}

// ---------------------------------------------------------------------------
// isTrustedProxy helper
// ---------------------------------------------------------------------------

func TestIsTrustedProxy(t *testing.T) {
	t.Parallel()

	proxies := MustParseCIDRs([]string{"10.0.0.0/8", "192.168.0.0/16"})

	cases := []struct {
		ip      string
		trusted bool
	}{
		{"10.1.2.3", true},
		{"192.168.5.10", true},
		{"172.16.0.1", false},
		{"1.2.3.4", false},
		{"8.8.8.8", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("ip=%s", tc.ip), func(t *testing.T) {
			t.Parallel()
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("could not parse test IP %s", tc.ip)
			}
			got := isTrustedProxy(ip, proxies)
			if got != tc.trusted {
				t.Fatalf("isTrustedProxy(%s) = %v, want %v", tc.ip, got, tc.trusted)
			}
		})
	}
}
