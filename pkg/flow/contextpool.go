package flow

import (
    "net/http"
    "sync"
)

// ContextPool centralizes allocation/reuse of *Context instances for an App.
// It wraps a sync.Pool and provides Get/Put helpers that reset fields as
// needed. Creating a pool per-App allows tuning or isolation if desired.
type ContextPool struct {
    pool sync.Pool
}

// NewContextPool creates a ready-to-use ContextPool.
func NewContextPool() *ContextPool {
    p := &ContextPool{}
    p.pool.New = func() any { return &Context{} }
    return p
}

// Get returns a Context from the pool, initializing fields for the given
// request. It never returns nil.
func (p *ContextPool) Get(app *App, w http.ResponseWriter, r *http.Request) *Context {
    if p == nil {
        return &Context{App: app, W: w, R: r}
    }
    v := p.pool.Get()
    if v == nil {
        return &Context{App: app, W: w, R: r}
    }
    c := v.(*Context)
    c.App = app
    c.W = w
    c.R = r
    c.status = 0
    c.rg = nil
    return c
}

// Put returns the Context to the pool after clearing request-scoped fields
// and waiting for any spawned request goroutines.
func (p *ContextPool) Put(c *Context) {
    if c == nil || p == nil {
        return
    }
    if c.rg != nil {
        _ = c.rg.Wait()
        c.rg = nil
    }
    // clear references
    c.App = nil
    c.W = nil
    c.R = nil
    c.status = 0
    p.pool.Put(c)
}
