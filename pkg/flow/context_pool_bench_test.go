package flow

import "testing"

// prevent compiler optimizing away allocations
var benchSink *Context

// Microbenchmarks to compare allocating Context directly vs using the
// pool-backed NewContext + PutContext.
func BenchmarkContextPool(b *testing.B) {
	b.Run("alloc_direct", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			c := &Context{}
			benchSink = c
		}
	})

	b.Run("pooled_new_put", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			c := NewContext(nil, nil, nil)
			benchSink = c
			PutContext(c)
		}
	})
}

// Benchmark that toggles UseContextPool to demonstrate the allocation
// difference when the pool is enabled vs disabled using the real
// NewContext/PutContext path.
func BenchmarkContextPoolToggle(b *testing.B) {
	b.Run("pool_enabled", func(b *testing.B) {
		UseContextPool = true
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			c := NewContext(nil, nil, nil)
			PutContext(c)
		}
	})

	b.Run("pool_disabled", func(b *testing.B) {
		// ensure we restore the global flag after the benchmark
		prev := UseContextPool
		UseContextPool = false
		defer func() { UseContextPool = prev }()

		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			c := NewContext(nil, nil, nil)
			PutContext(c)
		}
	})
}
