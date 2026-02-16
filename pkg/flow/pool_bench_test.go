package flow

import (
	"bytes"
	"net/http/httptest"
	"testing"
)

// These microbenchmarks compare allocations per request when using the
// Context pool versus allocating new Contexts on each request.

func BenchmarkContext_NoPool(b *testing.B) {
	old := UseContextPool
	UseContextPool = false
	defer func() { UseContextPool = old }()

	app := New("bench-no-pool")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", bytes.NewBuffer(nil))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := NewContext(app, w, r)
		PutContext(c)
	}
}

func BenchmarkContext_WithPool(b *testing.B) {
	old := UseContextPool
	UseContextPool = true
	defer func() { UseContextPool = old }()

	app := New("bench-with-pool")
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", bytes.NewBuffer(nil))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := NewContext(app, w, r)
		PutContext(c)
	}
}
