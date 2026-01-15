package flow

import (
    "net/http"
    "strings"
)

// SessionCookieOptions controls how Set-Cookie headers are hardened.
type SessionCookieOptions struct {
    Secure   bool
    SameSite http.SameSite
}

// SessionCookieHardening ensures cookie Set-Cookie headers include conservative
// security attributes (Secure and SameSite) when missing. It is useful to
// enable when running behind TLS to harden cookies created by existing
// middleware without modifying their implementation.
func SessionCookieHardening(opts ...func(*SessionCookieOptions)) Middleware {
    o := &SessionCookieOptions{Secure: true, SameSite: http.SameSiteLaxMode}
    for _, fn := range opts {
        fn(o)
    }

    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            next.ServeHTTP(w, r)

            sc := w.Header()["Set-Cookie"]
            if len(sc) == 0 {
                return
            }
            out := make([]string, 0, len(sc))
            for _, c := range sc {
                s := c
                low := strings.ToLower(s)
                if o.Secure && !strings.Contains(low, "secure") {
                    s = s + "; Secure"
                    low = strings.ToLower(s)
                }
                if o.SameSite != http.SameSiteDefaultMode && !strings.Contains(low, "samesite") {
                    var ss string
                    switch o.SameSite {
                    case http.SameSiteLaxMode:
                        ss = "Lax"
                    case http.SameSiteStrictMode:
                        ss = "Strict"
                    case http.SameSiteNoneMode:
                        ss = "None"
                    default:
                        ss = ""
                    }
                    if ss != "" {
                        s = s + "; SameSite=" + ss
                    }
                }
                out = append(out, s)
            }
            // replace Set-Cookie headers with hardened versions
            w.Header().Del("Set-Cookie")
            for _, v := range out {
                w.Header().Add("Set-Cookie", v)
            }
        })
    }
}
