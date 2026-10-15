package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/avalak/kv"
	"github.com/avalak/kv/memory"
)

// TestNoExpiryStores proves a NoExpiry entry survives a wall-clock delay
// that would have expired any short TTL, and that Has observes it. The
// generic contract only checks Save+Load without advance; the memory
// backend's expiry path needs the delay to be exercised.
func TestNoExpiryStores(t *testing.T) {
	b := memory.New[string, string](memory.Config[string]{Shards: 1, MaxItems: 10})
	ctx := context.Background()

	if err := b.Save(ctx, "k", "v", kv.NoExpiry); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if _, err := b.Load(ctx, "k"); err != nil {
		t.Fatalf("NoExpiry entry must not expire: %v", err)
	}
	if ok, _ := b.Has(ctx, "k"); !ok {
		t.Fatal("NoExpiry entry must be visible to Has")
	}
}

func TestNoExpiryOverwrittenByShortTTL(t *testing.T) {
	b := memory.New[string, string](memory.Config[string]{Shards: 1, MaxItems: 10})
	ctx := context.Background()

	if err := b.Save(ctx, "k", "v", kv.NoExpiry); err != nil {
		t.Fatal(err)
	}
	if err := b.Save(ctx, "k", "v", 20*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if _, err := b.Load(ctx, "k"); err == nil {
		t.Fatal("short TTL after NoExpiry must expire")
	}
}
