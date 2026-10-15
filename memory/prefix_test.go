package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/avalak/kv"
	"github.com/avalak/kv/memory"
)

func TestDropPrefix(t *testing.T) {
	b := memory.New[string, string](memory.Config[string]{Shards: 4, MaxItems: 128})
	ctx := context.Background()
	for _, k := range []string{"user:1", "user:2", "user:3", "session:1", "session:2"} {
		_ = b.Save(ctx, k, "v", time.Minute)
	}

	if err := b.DropPrefix(ctx, "user:"); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"user:1", "user:2", "user:3"} {
		if ok, _ := b.Has(ctx, k); ok {
			t.Fatalf("%s survived DropPrefix", k)
		}
	}
	for _, k := range []string{"session:1", "session:2"} {
		if ok, _ := b.Has(ctx, k); !ok {
			t.Fatalf("%s removed by unrelated prefix", k)
		}
	}
}

func TestDropPrefixEmpty(t *testing.T) {
	b := memory.New[string, string](memory.Config[string]{Shards: 2, MaxItems: 32})
	ctx := context.Background()
	_ = b.Save(ctx, "a", "v", time.Minute)
	_ = b.Save(ctx, "b", "v", time.Minute)

	if err := b.DropPrefix(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if ok, _ := b.Has(ctx, "a"); ok {
		t.Fatal("empty prefix must match every key")
	}
	if ok, _ := b.Has(ctx, "b"); ok {
		t.Fatal("empty prefix must match every key")
	}
}

// TestDropPrefixUnsupportedKeyType pins the contract for a backend whose
// key type cannot be matched as a string.
func TestDropPrefixUnsupportedKeyType(t *testing.T) {
	b := memory.New[structKey, string](memory.Config[string]{Shards: 2, MaxItems: 32})
	err := b.DropPrefix(context.Background(), structKey{"x", 1})
	if !errors.Is(err, kv.ErrPrefixUnsupported) {
		t.Fatalf("want ErrPrefixUnsupported, got %v", err)
	}
}
