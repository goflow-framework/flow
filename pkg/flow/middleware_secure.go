package flow

import (
    "crypto/tls"
    "fmt"
    "net/http"
    "time"
)

// SecureHeadersOptions configures the behavior of the SecureHeaders middleware.
type SecureHeadersOptions struct {
    // HSTSMaxAge controls the Strict-Transport-Security max-age. A zero
    // duration disables HSTS.
    HSTSMaxAge time.Duration
    // HSTSIncludeSubdomains appends IncludeSubDomains when true.
    HSTSIncludeSubdomains bool
    // HSTSPreload appends preload when true.
    HSTSPreload bool

    // ContentSecurityPolicy sets the Content-Security-Policy header when not
    // empty. Left empty by default to avoid breaking existing apps.
    ContentSecurityPolicy string

    // ReferrerPolicy sets the Referrer-Policy header. If empty a reasonable
    // default is used.
    ReferrerPolicy string

    // PermissionsPolicy sets the Permissions-Policy header when not empty.
    PermissionsPolicy string
}

// SecureHeaders returns a middleware that applies a conservative set of
// security-related response headers. It is safe to call with no options.
func SecureHeaders(opts ...func(*SecureHeadersOptions)) Middleware {
    // default options
    o := &SecureHeadersOptions{
        HSTSMaxAge:   30 * 24 * time.Hour, // 30 days
        ReferrerPolicy: "strict-origin-when-cross-origin",
    }
    for _, fn := range opts {
        fn(o)
    }

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // HSTS: only set when request appears to be over TLS
            if o.HSTSMaxAge > 0 && isTLS(r) {
                val := fmt.Sprintf("max-age=%d", int(o.HSTSMaxAge.Seconds()))
                if o.HSTSIncludeSubdomains {
                    val += "; includeSubDomains"
                }
                if o.HSTSPreload {
                    val += "; preload"
                }
                w.Header().Set("Strict-Transport-Security", val)
            }

            // Common hardening headers
            w.Header().Set("X-Frame-Options", "DENY")
            w.Header().Set("X-Content-Type-Options", "nosniff")

            if o.ReferrerPolicy != "" {
                w.Header().Set("Referrer-Policy", o.ReferrerPolicy)
            }
            if o.PermissionsPolicy != "" {
                w.Header().Set("Permissions-Policy", o.PermissionsPolicy)
            }
            if o.ContentSecurityPolicy != "" {
                w.Header().Set("Content-Security-Policy", o.ContentSecurityPolicy)
            }

            next.ServeHTTP(w, r)
        })
    }
}

// WithSecureDefaults is a convenience helper that registers the conservative
// secure middleware set onto the provided App. It returns nil for call-site
// chaining convenience.
func WithSecureDefaults(a *App, opts ...func(*SecureHeadersOptions)) error {
    if a == nil {
        return nil
    }
    // register secure headers
    a.Use(SecureHeaders(opts...))
    // add session cookie hardening middleware so existing Set-Cookie headers
    // get conservative attributes (Secure; SameSite=Lax) when missing
    a.Use(SessionCookieHardening())
    // if an app session manager exists, enable conservative cookie defaults
    if a.Sessions != nil {
        a.Sessions.ApplySecureCookieDefaults()
    }
    return nil
}

func isTLS(r *http.Request) bool {
    if r == nil {
        return false
    }
    if r.TLS != nil {
        // non-nil TLS means request arrived over TLS
        return true
    }
    // Some tests set a header to indicate TLS; consider X-Forwarded-Proto
    // when behind proxies. We treat "https" as TLS.
    if proto := r.Header.Get("X-Forwarded-Proto"); proto == "https" {
        return true
    }
    // Fallback for tests that assign a tls.ConnectionState value to Request.TLS
    // (already handled above), keep this for clarity.
    _ = tls.ConnectionState{}
    return false
}
