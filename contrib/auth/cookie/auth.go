package cookie

import (
	"context"
	"net/http"

	flow "github.com/goflow-framework/flow/pkg/flow"
)

// ctxKey is an unexported type for storing the current user in request context.
type ctxKey struct{}

// Middleware returns a flow.Middleware that looks up a user identifier from the
// session (sessionKey) and resolves it using the provided lookup function.
// If lookup returns an error the middleware will treat the user as unauthenticated.
// The resolved user (any value) is stored in the request context and can be
// retrieved via FromContext.
func Middleware(sm *flow.SessionManager, sessionKey string, lookup func(id interface{}) (interface{}, error)) flow.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Attach default nil user
			var user interface{}

			// Attempt to extract session and read the value
			if s := flow.FromContext(r.Context()); s != nil {
				if v, ok := s.Get(sessionKey); ok {
					if lookup != nil {
						if u, err := lookup(v); err == nil {
							user = u
						}
					}
				}
			}

			ctx := context.WithValue(r.Context(), ctxKey{}, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// FromContext returns the resolved user value previously stored by the
// Middleware. The boolean is false when no user is present.
func FromContext(ctx context.Context) (interface{}, bool) {
	if ctx == nil {
		return nil, false
	}
	v := ctx.Value(ctxKey{})
	if v == nil {
		return nil, false
	}
	return v, true
}
