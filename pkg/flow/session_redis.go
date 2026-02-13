// Redis-backed session store adapter.
//
// This file implements a Redis-backed session store and manager used by
// the examples and tests. It previously required the "redis" build tag;
// the implementations are now included in normal builds so examples such
// as `examples/redis-session` compile out of the box.

package flow

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore is a thin wrapper around a go-redis client that persists
// session maps under keys like "flow:session:<id>".
type RedisStore struct {
	client *redis.Client
	prefix string
}

// NewRedisStore creates a new RedisStore. The caller is responsible for
// creating the redis.Options and managing the lifecycle of the client
// (closing it if needed).
func NewRedisStore(opts *redis.Options, prefix string) *RedisStore {
	if prefix == "" {
		prefix = "flow:session:"
	}
	return &RedisStore{client: redis.NewClient(opts), prefix: prefix}
}

func (rs *RedisStore) key(id string) string { return rs.prefix + id }

func (rs *RedisStore) Load(ctx context.Context, id string) (map[string]interface{}, error) {
	b, err := rs.client.Get(ctx, rs.key(id)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return map[string]interface{}{}, nil
		}
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (rs *RedisStore) Save(ctx context.Context, id string, values map[string]interface{}, ttl time.Duration) error {
	b, err := json.Marshal(values)
	if err != nil {
		return err
	}
	return rs.client.Set(ctx, rs.key(id), b, ttl).Err()
}

func (rs *RedisStore) Delete(ctx context.Context, id string) error {
	return rs.client.Del(ctx, rs.key(id)).Err()
}

// RedisSessionManager implements a session manager which stores the
// session values in Redis and keeps only a signed session id in the cookie.
type RedisSessionManager struct {
	secret     []byte
	CookieName string
	MaxAge     int
	Store      *RedisStore
}

// NewRedisSessionManager creates a manager backed by the provided store.
func NewRedisSessionManager(secret []byte, cookieName string, store *RedisStore) *RedisSessionManager {
	if cookieName == "" {
		cookieName = "flow_session"
	}
	return &RedisSessionManager{secret: secret, CookieName: cookieName, MaxAge: 86400, Store: store}
}

// signID returns hex(signature) of id using hmac-sha256 with the manager secret.
func (rsm *RedisSessionManager) signID(id string) string {
	mac := hmac.New(sha256.New, rsm.secret)
	mac.Write([]byte(id))
	return hex.EncodeToString(mac.Sum(nil))
}

// Middleware loads a session from redis (if present) and places a Session
// object in the request context. Save/Delete on Session must be implemented
// by wiring into the redis store; for now we provide a minimal scaffold.
func (rsm *RedisSessionManager) Middleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// read cookie, verify signature, load values from store
			c, err := r.Cookie(rsm.CookieName)
			var vals map[string]interface{}
			var id string
			if err == nil {
				parts := strings.Split(c.Value, "|")
				if len(parts) == 2 {
					id = parts[0]
					sig := parts[1]
					if rsm.signID(id) == sig {
						// safe to load
						vals, _ = rsm.Store.Load(r.Context(), id)
					}
				}
			}
			if vals == nil {
				vals = map[string]interface{}{}
			}
			// create a Session-like wrapper that delegates Save/Delete to redis
			s := &redisBackedSession{values: vals, rsm: rsm, id: id, w: w, r: r}
			ctx := context.WithValue(r.Context(), sessionCtxKey{}, s)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// redisBackedSession implements a subset of the Session API and persists
// values into Redis when Save/Delete/Set are called.
type redisBackedSession struct {
	values map[string]interface{}
	rsm    *RedisSessionManager
	id     string
	w      http.ResponseWriter
	r      *http.Request
}

func (s *redisBackedSession) Get(key string) (interface{}, bool) {
	v, ok := s.values[key]
	return v, ok
}
func (s *redisBackedSession) Set(key string, v interface{}) error { s.values[key] = v; return s.Save() }
func (s *redisBackedSession) Delete(key string) error             { delete(s.values, key); return s.Save() }

func (s *redisBackedSession) Save() error {
	// If no id assigned, generate one
	if s.id == "" {
		idb := make([]byte, 16)
		if _, err := rand.Read(idb); err != nil {
			return err
		}
		s.id = hex.EncodeToString(idb)
	}
	// set cookie: id|sig
	sig := s.rsm.signID(s.id)
	cookie := &http.Cookie{
		Name:     s.rsm.CookieName,
		Value:    s.id + "|" + sig,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		Expires:  time.Now().Add(time.Duration(s.rsm.MaxAge) * time.Second),
		MaxAge:   s.rsm.MaxAge,
	}
	http.SetCookie(s.w, cookie)
	// persist into redis
	return s.rsm.Store.Save(s.r.Context(), s.id, s.values, time.Duration(s.rsm.MaxAge)*time.Second)
}

func (s *redisBackedSession) DeleteAll() error {
	if s.id == "" {
		return nil
	}
	return s.rsm.Store.Delete(s.r.Context(), s.id)
}
