package flow

import (
    "context"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "net/http"
    "time"
)

// SessionStore defines an adapter surface for different session backends.
// The interface operates in terms of HTTP requests/responses because cookie
// backed stores need access to the request/response while server-backed
// stores (Redis) persist data by id.
type SessionStore interface {
    // LoadRequest loads session values for the incoming request. It returns
    // the map of values and the session id (may be empty for cookie stores).
    LoadRequest(r *http.Request) (map[string]interface{}, string, error)

    // SaveResponse persists values and ensures the response contains any
    // cookies necessary to identify the session. If the store assigns a
    // session id it should be returned.
    SaveResponse(w http.ResponseWriter, r *http.Request, id string, values map[string]interface{}, ttl time.Duration) (string, error)

    // DeleteResponse removes any server-side session state and clears
    // identifying cookies from the response.
    DeleteResponse(w http.ResponseWriter, r *http.Request, id string) error
}

// CookieStore implements SessionStore by storing the entire session map
// as a signed cookie. It is simple and requires no server-side storage.
type CookieStore struct {
    sm *SessionManager
}

// NewCookieStore creates a cookie-backed session store reusing the
// existing SessionManager implementation.
func NewCookieStore(secret []byte, cookieName string, maxAge int) *CookieStore {
    return &CookieStore{sm: NewSessionManager(secret, cookieName)}
}

func (cs *CookieStore) LoadRequest(r *http.Request) (map[string]interface{}, string, error) {
    vals, err := cs.sm.loadFromRequest(r)
    return vals, "", err
}

func (cs *CookieStore) SaveResponse(w http.ResponseWriter, r *http.Request, id string, values map[string]interface{}, ttl time.Duration) (string, error) {
    // SessionManager.Save uses MaxAge from the manager and writes cookie.
    s := &Session{values: values, sm: cs.sm, w: w, r: r}
    if err := s.Save(); err != nil {
        return "", err
    }
    return "", nil
}

func (cs *CookieStore) DeleteResponse(w http.ResponseWriter, r *http.Request, id string) error {
    // Clear cookie
    cookie := &http.Cookie{
        Name:     cs.sm.CookieName,
        Value:    "",
        Path:     "/",
        HttpOnly: true,
        Secure:   false,
        Expires:  time.Unix(0, 0),
        MaxAge:   -1,
    }
    http.SetCookie(w, cookie)
    return nil
}

// RedisStoreAdapter adapts the existing RedisStore/RedisSessionManager
// semantics into the SessionStore interface by using an HMAC-signed id
// placed into a cookie. It reuses logic similar to RedisSessionManager.
type RedisStoreAdapter struct {
    Store      *RedisStore
    Secret     []byte
    CookieName string
    MaxAge     int
}

// NewRedisStoreAdapter creates an adapter that will sign ids placed in a cookie.
func NewRedisStoreAdapter(secret []byte, cookieName string, store *RedisStore) *RedisStoreAdapter {
    if cookieName == "" {
        cookieName = "flow_session"
    }
    return &RedisStoreAdapter{Store: store, Secret: secret, CookieName: cookieName, MaxAge: 86400}
}

func (rsm *RedisStoreAdapter) signID(id string) string {
    mac := hmac.New(sha256.New, rsm.Secret)
    mac.Write([]byte(id))
    return hex.EncodeToString(mac.Sum(nil))
}

func (rsm *RedisStoreAdapter) LoadRequest(r *http.Request) (map[string]interface{}, string, error) {
    c, err := r.Cookie(rsm.CookieName)
    if err != nil {
        return map[string]interface{}{}, "", nil
    }
    parts := splitCookieParts(c.Value)
    if len(parts) != 2 {
        return map[string]interface{}{}, "", nil
    }
    id := parts[0]
    sig := parts[1]
    if rsm.signID(id) != sig {
        return map[string]interface{}{}, "", nil
    }
    vals, err := rsm.Store.Load(r.Context(), id)
    return vals, id, err
}

func (rsm *RedisStoreAdapter) SaveResponse(w http.ResponseWriter, r *http.Request, id string, values map[string]interface{}, ttl time.Duration) (string, error) {
    // generate id if needed
    if id == "" {
        // lightweight id generation — duplicate from existing code
        b := make([]byte, 16)
        if _, err := rand.Read(b); err != nil {
            return "", err
        }
        id = hex.EncodeToString(b)
    }
    // set cookie id|sig
    sig := rsm.signID(id)
    cookie := &http.Cookie{
        Name:     rsm.CookieName,
        Value:    id + "|" + sig,
        Path:     "/",
        HttpOnly: true,
        Secure:   false,
        Expires:  time.Now().Add(time.Duration(rsm.MaxAge) * time.Second),
        MaxAge:   rsm.MaxAge,
    }
    http.SetCookie(w, cookie)
    if err := rsm.Store.Save(r.Context(), id, values, ttl); err != nil {
        return "", err
    }
    return id, nil
}

func (rsm *RedisStoreAdapter) DeleteResponse(w http.ResponseWriter, r *http.Request, id string) error {
    if id == "" {
        // nothing to delete
        return nil
    }
    // clear cookie
    cookie := &http.Cookie{
        Name:     rsm.CookieName,
        Value:    "",
        Path:     "/",
        HttpOnly: true,
        Secure:   false,
        Expires:  time.Unix(0, 0),
        MaxAge:   -1,
    }
    http.SetCookie(w, cookie)
    return rsm.Store.Delete(r.Context(), id)
}

// splitCookieParts splits value of the form "id|sig".
func splitCookieParts(v string) []string {
    parts := make([]string, 0, 2)
    for i, c := range v {
        _ = i
        _ = c
    }
    // simple split
    idx := -1
    for i := 0; i < len(v); i++ {
        if v[i] == '|' {
            idx = i
            break
        }
    }
    if idx == -1 {
        return []string{v}
    }
    return []string{v[:idx], v[idx+1:]}
}
