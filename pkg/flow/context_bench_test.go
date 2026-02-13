package flow

import (
    "testing"
)

// Microbenchmarks for Context allocation with and without pooling.
func BenchmarkNewContext_NoPool(b *testing.B) {
    prev := UseContextPool
    UseContextPool = false
    defer func() { UseContextPool = prev }()

    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        c := NewContext(nil, nil, nil)
        PutContext(c)
    }
}

func BenchmarkNewContext_PackagePool(b *testing.B) {
    prev := UseContextPool
    UseContextPool = true
    defer func() { UseContextPool = prev }()

    // ensure package pool is primed
    for i := 0; i < 10; i++ {
        contextPool.Put(&Context{})
    }

    b.ReportAllocs()
    for i := 0; i < b.N; i++ {
        c := NewContext(nil, nil, nil)
        PutContext(c)
    }
}

func BenchmarkNewContext_AppPool(b *testing.B) {
    prev := UseContextPool
    UseContextPool = true
    defer func() { UseContextPool = prev }()

    // Create the app and prime the pool outside the timed section so we
    // measure only Get/Put overhead.
    app := New("bench-app")
    // prime the app pool with a few entries
    for i := 0; i < 100; i++ {
        app.ctxPool.pool.Put(&Context{})
    }

    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        c := app.ctxPool.Get(app, nil, nil)
        app.ctxPool.Put(c)
    }
}
