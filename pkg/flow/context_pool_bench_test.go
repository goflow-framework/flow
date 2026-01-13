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
