package kv_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/avalak/kv"
)

// noPrefixBackend is a Backend without DropPrefix, used to exercise the
// Cache.DeletePrefix fallback.
type noPrefixBackend[K comparable, V any] struct{}

func (noPrefixBackend[K, V]) Load(context.Context, K) (V, error) {
	var zero V
	return zero, kv.ErrNotFound
}

func (noPrefixBackend[K, V]) Save(context.Context, K, V, time.Duration) error { return nil }
func (noPrefixBackend[K, V]) Drop(context.Context, K) error                   { return nil }
func (noPrefixBackend[K, V]) Has(context.Context, K) (bool, error)            { return false, nil }
func (noPrefixBackend[K, V]) Close() error                                    { return nil }

func TestCacheDeletePrefix(t *testing.T) {
	c := newTestCache(t, clonePayload)
	ctx := context.Background()
	for _, k := range []string{"user:1", "user:2", "session:1"} {
		_ = c.Put(ctx, k, &payload{Data: []byte(k)}, time.Minute)
	}

	if err := c.DeletePrefix(ctx, "user:"); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"user:1", "user:2"} {
		if ok, _ := c.Has(ctx, k); ok {
			t.Fatalf("%s survived DeletePrefix", k)
		}
	}
	if ok, _ := c.Has(ctx, "session:1"); !ok {
		t.Fatal("session:1 removed by unrelated prefix")
	}
}

func TestCacheDeletePrefixUnsupported(t *testing.T) {
	c := kv.New[string, *payload](noPrefixBackend[string, *payload]{}, nil)
	err := c.DeletePrefix(context.Background(), "x")
	if !errors.Is(err, kv.ErrPrefixUnsupported) {
		t.Fatalf("want ErrPrefixUnsupported, got %v", err)
	}
}
