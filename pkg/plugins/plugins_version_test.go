package plugins

import (
	"context"
	"testing"

	"github.com/undiegomejia/flow/pkg/flow"
)

// incompatiblePlugin reports a different major version than the framework's
type incompatiblePlugin struct{}

func (p *incompatiblePlugin) Name() string                    { return "bad" }
func (p *incompatiblePlugin) Version() string                 { return "v1.2.3" }
func (p *incompatiblePlugin) Init(a *flow.App) error          { return nil }
func (p *incompatiblePlugin) Mount(a *flow.App) error         { return nil }
func (p *incompatiblePlugin) Middlewares() []flow.Middleware  { return nil }
func (p *incompatiblePlugin) Start(ctx context.Context) error { return nil }
func (p *incompatiblePlugin) Stop(ctx context.Context) error  { return nil }

func TestRegister_IncompatibleVersionReturnsError(t *testing.T) {
	Reset()
	err := Register(&incompatiblePlugin{})
	if err == nil {
		t.Fatalf("expected error when registering plugin with incompatible major version")
	}
}
