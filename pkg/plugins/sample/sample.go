package sample

import (
	"context"

	"github.com/goflow-framework/flow/pkg/flow"
	"github.com/goflow-framework/flow/pkg/plugins"
)

type samplePlugin struct{}

func (p *samplePlugin) Name() string    { return "sample" }
func (p *samplePlugin) Version() string { return "v0.0.1" }

func (p *samplePlugin) Init(a *flow.App) error {
	// register a small service visible to the App
	_ = a.RegisterService("sample.value", "hello-from-sample-plugin")
	return nil
}

func (p *samplePlugin) Mount(a *flow.App) error {
	// no-op mount for the sample plugin
	return nil
}

func (p *samplePlugin) Middlewares() []flow.Middleware { return nil }

func (p *samplePlugin) Start(ctx context.Context) error { return nil }
func (p *samplePlugin) Stop(ctx context.Context) error  { return nil }

func init() {
	// register at compile time for examples that import this package
	_ = plugins.Register(&samplePlugin{})
}
