package simple

import "github.com/undiegomejia/flow/pkg/flow"

// Register registers a simple service into the provided App. This
// demonstrates the recommended runtime registration pattern.
func Register(app *flow.App) error {
    if app == nil {
        return nil
    }
    // register a trivial service; in a real plugin this could be an
    // interface-backed mailer, metrics client, etc.
    _ = app.RegisterService("example.simple", "hello-from-simple-plugin")
    return nil
}
