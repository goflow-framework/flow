package flow

import (
    "net/http"
)

// CSRFMiddlewareJSON returns a middleware targeted for JSON APIs. It validates
// unsafe HTTP methods by requiring the X-CSRF-Token header to be present and
// equal to the session's CSRF token. This is a lightweight helper suitable
// for single-page apps or API clients that transmit the token via a header.
func CSRFMiddlewareJSON() Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Only validate unsafe methods
            if isUnsafeMethod(r.Method) {
                // Only validate JSON requests here; other content types should use
                // the form-aware CSRFMiddleware() which also parses form fields.
                ct := r.Header.Get("Content-Type")
                if ct == "" || (!hasJSONContentType(ct)) {
                    // Not a JSON request; skip here and let form CSRFMiddleware
                    // handle it if registered.
                    next.ServeHTTP(w, r)
                    return
                }

                // Check token header
                header := r.Header.Get("X-CSRF-Token")
                if !secureCompare(header, CSRFToken(r)) {
                    http.Error(w, "forbidden", http.StatusForbidden)
                    return
                }
            }
            next.ServeHTTP(w, r)
        })
    }
}

// hasJSONContentType is a small helper to detect common JSON content types.
func hasJSONContentType(ct string) bool {
    // content type may include a charset, so check prefix
    return len(ct) >= 16 && (ct == "application/json" || ct[:16] == "application/json")
}
