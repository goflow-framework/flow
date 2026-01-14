package main

import (
    "context"
    "fmt"
    "log"

    "github.com/undiegomejia/flow/pkg/flow"
    "github.com/undiegomejia/flow/pkg/plugins"
)

// runtimePlugin demonstrates registering a plugin at runtime from main.
type runtimePlugin struct{}

func (p *runtimePlugin) Name() string    { return "runtime-sample" }
func (p *runtimePlugin) Version() string { return "v0.0.1" }
func (p *runtimePlugin) Init(a *flow.App) error {
    // register a service so main can observe it after ApplyAll
    return a.RegisterService("runtime.value", "hello-from-runtime")
}
func (p *runtimePlugin) Mount(a *flow.App) error                   { return nil }
func (p *runtimePlugin) Middlewares() []flow.Middleware            { return nil }
func (p *runtimePlugin) Start(ctx context.Context) error { return nil }
func (p *runtimePlugin) Stop(ctx context.Context) error  { return nil }

func main() {
    app := flow.New("runtime-demo")

    // register plugin at runtime instead of compile-time init()
    if err := plugins.Register(&runtimePlugin{}); err != nil {
        log.Fatalf("register runtime plugin: %v", err)
    }

    if err := plugins.ApplyAll(app); err != nil {
        log.Fatalf("apply plugins: %v", err)
    }

    if v, ok := app.GetService("runtime.value"); ok {
        fmt.Printf("runtime.value=%v\n", v)
    } else {
        fmt.Println("runtime.value not found")
    }

    _ = app.Shutdown(context.Background())
}
