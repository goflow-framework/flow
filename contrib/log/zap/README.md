# Zap adapter for Flow

This package provides a small adapter to use an Uber `zap` logger with the
Flow framework's logging abstractions.

Usage
-----

```go
import (
    "go.uber.org/zap"
    zapadapter "github.com/goflow-framework/flow/contrib/log/zap"
    "github.com/goflow-framework/flow/pkg/flow"
)

func main() {
    z, _ := zap.NewProduction()
    defer z.Sync()

    app := flow.New("my-app", flow.WithLogger(zapadapter.NewZapAdapter(z)))
    // ... configure routes and start app
    _ = app
}
```

Notes
-----
- `NewZapAdapter` will use a noop zap logger when passed `nil` to avoid
  panics during tests or when no logger is available.
