package flow

import (
	"context"
	"testing"
)

// Benchmarks comparing explicit Wait() vs relying on PutContext() to wait.
// We run each benchmark with the Context pool enabled and disabled to show
// allocation differences.
func BenchmarkRequestGroupWaitComparison(b *testing.B) {
	for _, pool := range []struct {
		name string
		on   bool
	}{
		{"pool_enabled", true},
		{"pool_disabled", false},
	} {
		b.Run(pool.name, func(b *testing.B) {
			prev := UseContextPool
			UseContextPool = pool.on
			defer func() { UseContextPool = prev }()

			b.Run("explicit_wait", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					c := NewContext(nil, nil, nil)
					c.Go(func(ctx context.Context) error { return nil })
					// handler explicitly waits
					_ = c.RequestGroup().Wait()
					PutContext(c)
				}
			})

			b.Run("implicit_wait", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					c := NewContext(nil, nil, nil)
					c.Go(func(ctx context.Context) error { return nil })
					// handler returns and deferred PutContext will wait
					PutContext(c)
				}
			})
		})
	}
}
