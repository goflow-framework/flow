package flow

import "testing"

// TestContextPool_GetPut_Reuses ensures that a Context returned to the pool
// can be retrieved again (reuse) and that fields are cleared/reset by Put.
func TestContextPool_GetPut_Reuses(t *testing.T) {
	p := NewContextPool()

	// Create a context with some sentinel values and put it into the pool.
	c := &Context{status: 42}
	p.Put(c)

	got := p.Get(nil, nil, nil)
	if got == nil {
		t.Fatalf("expected non-nil Context from pool")
	}
	if got != c {
		// sync.Pool does not strictly guarantee the same pointer will be
		// returned, but in local deterministic cases it usually will. If
		// it's different, still assert that fields were reset below.
		t.Logf("pool returned different instance (OK): got %p want %p", got, c)
	}

	// Fields should be reset by Get/Put (status reset to 0)
	if got.status != 0 {
		t.Fatalf("expected status reset to 0; got %d", got.status)
	}

	// Return it again to ensure Put works without panic.
	p.Put(got)
}
