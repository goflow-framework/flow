// This file contains the App lifecycle methods: Start, Run, Shutdown and the
// plugin fan-out helpers. It delegates the HTTP server state machine to the
// internal/app.Lifecycle type, keeping pkg/flow/app.go focused on wiring.
package flow

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	internalapp "github.com/undiegomejia/flow/internal/app"
)

// startPlugins invokes Start(ctx) for each registered plugin in registration
// order in its own goroutine. Errors returned by Start are logged. The
// provided ctx is canceled by App.Shutdown (via pluginCancel) to signal
// plugin goroutines to exit.
func (a *App) startPlugins(ctx context.Context) {
	if a == nil {
		return
	}
	a.pluginsMu.RLock()
	names := make([]string, len(a.pluginOrder))
	copy(names, a.pluginOrder)
	a.pluginsMu.RUnlock()

	for _, name := range names {
		a.pluginsMu.RLock()
		p := a.plugins[name]
		a.pluginsMu.RUnlock()
		if p == nil {
			continue
		}
		a.pluginWg.Add(1)
		go func(p Plugin, n string) {
			defer a.pluginWg.Done()
			if err := p.Start(ctx); err != nil {
				a.Logger().Printf("plugin %s start error: %v", n, err)
			}
		}(p, name)
	}
}

// shutdownPlugins invokes Stop(ctx) on registered plugins in reverse
// registration order and aggregates errors.
func (a *App) shutdownPlugins(ctx context.Context) error {
	if a == nil {
		return nil
	}
	a.pluginsMu.RLock()
	names := make([]string, len(a.pluginOrder))
	copy(names, a.pluginOrder)
	a.pluginsMu.RUnlock()

	var errs []string
	for i := len(names) - 1; i >= 0; i-- {
		name := names[i]
		a.pluginsMu.RLock()
		p := a.plugins[name]
		a.pluginsMu.RUnlock()
		if p == nil {
			continue
		}
		if err := p.Stop(ctx); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("plugin shutdown error: %s", errs[0])
	default:
		return fmt.Errorf("plugin shutdown errors: %s", strings.Join(errs, "; "))
	}
}

// Start starts the HTTP server in a background goroutine and returns
// immediately. It returns ErrAppAlreadyRunning if called while the server is
// already running.
func (a *App) Start() error {
	if !atomic.CompareAndSwapInt32(&a.state, 0, 1) {
		return ErrAppAlreadyRunning
	}

	a.lc = internalapp.NewLifecycle(a.Handler(), a.Addr, a.ReadTimeout, a.WriteTimeout, a.IdleTimeout, a.logger)

	// create a plugin context; canceled in Shutdown.
	a.pluginCtx, a.pluginCancel = context.WithCancel(context.Background())

	if err := a.lc.Start(); err != nil {
		// roll back state so the caller can retry
		atomic.StoreInt32(&a.state, 0)
		a.lc = nil
		return err
	}

	a.startPlugins(a.pluginCtx)
	return nil
}

// Run starts the server and blocks until a termination signal is received or
// the context is canceled, then performs a graceful shutdown.
func (a *App) Run(ctx context.Context) error {
	if err := a.Start(); err != nil {
		return err
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case <-ctx.Done():
		a.logger.Printf("context canceled, shutting down: %v", ctx.Err())
	case sig := <-sigCh:
		a.logger.Printf("received signal %s, shutting down", sig)
	}

	t := a.ShutdownTimeout
	if t <= 0 {
		t = 10 * time.Second
	}
	ctxShutdown, cancel := context.WithTimeout(context.Background(), t)
	defer cancel()

	return a.Shutdown(ctxShutdown)
}

// Shutdown gracefully stops the HTTP server. It is safe to call multiple times.
func (a *App) Shutdown(ctx context.Context) error {
	if a.lc == nil {
		return nil
	}
	if !atomic.CompareAndSwapInt32(&a.state, 1, 2) {
		if atomic.LoadInt32(&a.state) == 2 {
			return nil
		}
	}

	// Cancel plugin contexts and wait for goroutines to exit.
	if a.pluginCancel != nil {
		a.pluginCancel()
		done := make(chan struct{})
		go func() {
			a.pluginWg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	a.logger.Printf("shutting down %s", a.Name)

	// Delegate HTTP drain to the lifecycle engine.
	if err := a.lc.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	a.logger.Printf("shutdown complete")

	// Stop job workers.
	if len(a.workers) > 0 {
		a.workersMu.Lock()
		handles := make([]workerHandle, len(a.workers))
		copy(handles, a.workers)
		a.workersMu.Unlock()
		for _, h := range handles {
			h.w.Stop()
		}
		for _, h := range handles {
			select {
			case <-h.done:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	if a.executorShutdown != nil {
		_ = a.executorShutdown(ctx)
	}
	if a.tracerShutdown != nil {
		_ = a.tracerShutdown(ctx)
	}
	_ = a.shutdownPlugins(ctx)

	return nil
}
