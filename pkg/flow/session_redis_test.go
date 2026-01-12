//go:build redis
// +build redis

package flow

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisStore_SaveLoadDelete(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis run: %v", err)
	}
	defer s.Close()

	opts := &redis.Options{Addr: s.Addr()}
	store := NewRedisStore(opts, "test:session:")

	ctx := context.Background()
	id := "tid123"
	vals := map[string]interface{}{"foo": "bar"}

	if err := store.Save(ctx, id, vals, time.Minute); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := store.Load(ctx, id)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got["foo"] != "bar" {
		t.Fatalf("expected foo=bar got %v", got["foo"])
	}

	if err := store.Delete(ctx, id); err != nil {
		t.Fatalf("delete: %v", err)
	}

	got2, err := store.Load(ctx, id)
	if err != nil {
		t.Fatalf("load after delete: %v", err)
	}
	if len(got2) != 0 {
		t.Fatalf("expected empty after delete, got %v", got2)
	}
}
