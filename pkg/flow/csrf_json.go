package flow

import (
	"mime"
	"net/http"
	"strings"
)

// CSRFMiddlewareJSON returns a middleware targeted for JSON APIs. It validates
// unsafe HTTP methods by requiring the X-CSRF-Token header to be present and
// equal to the session's CSRF token. This is a lightweight helper suitable
// for single-page apps or API clients that transmit the token via a header.
//
// Like CSRFMiddleware, this variant requires SessionManager.Middleware() to be
// registered earlier in the stack. If no session is present in the request
// context the middleware responds with 500 and a diagnostic message.
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

				// Guard: session middleware must be registered before CSRFMiddlewareJSON.
				if FromContext(r.Context()) == nil {
					http.Error(w,
						"csrf: session middleware is not configured — "+
							"register SessionManager.Middleware() before CSRFMiddlewareJSON() in your middleware stack",
						http.StatusInternalServerError)
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
} // hasJSONContentType reports whether the Content-Type header value represents
// a JSON media type. It uses mime.ParseMediaType to correctly strip parameters
// (e.g. charset) before comparison, and accepts:
//   - application/json          (RFC 4627 / RFC 8259)
//   - any type ending in +json  (RFC 6839 structured-syntax suffix)
//
// Examples that return true:
//
//	"application/json"
//	"application/json; charset=utf-8"
//	"application/vnd.api+json"
//	"application/ld+json"
//	"application/problem+json"
func hasJSONContentType(ct string) bool {
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return false
	}
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}
