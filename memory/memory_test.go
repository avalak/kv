package memory_test

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/avalak/kv"
	"github.com/avalak/kv/kvtest"
	"github.com/avalak/kv/memory"
)

func TestContractString(t *testing.T) {
	kvtest.Contract[string, string](t,
		func() kvtest.Harness[string, string] {
			return kvtest.Harness[string, string]{
				Backend: memory.New[string, string](memory.Config[string]{
					Shards:   8,
					MaxItems: 1024,
					MaxBytes: 1 << 20,
					SizeOf:   func(s string) int { return len(s) },
				}),
			}
		},
		kvtest.KeySet[string]{Main: "k", Other: "other", Missing: "missing"},
		"hello",
		func(a, b string) bool { return a == b },
	)
}

func TestContractUint64(t *testing.T) {
	kvtest.Contract[uint64, int](t,
		func() kvtest.Harness[uint64, int] {
			return kvtest.Harness[uint64, int]{
				Backend: memory.New[uint64, int](memory.Config[int]{Shards: 4, MaxItems: 128}),
			}
		},
		kvtest.KeySet[uint64]{Main: 1, Other: 2, Missing: 999},
		42,
		func(a, b int) bool { return a == b },
	)
}

type structKey struct {
	Domain string
	QType  uint16
}

func structKeyHash(k structKey) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(k.Domain))
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], k.QType)
	_, _ = h.Write(b[:])
	return h.Sum64()
}

func TestContractStructKey(t *testing.T) {
	kvtest.Contract[structKey, string](t,
		func() kvtest.Harness[structKey, string] {
			return kvtest.Harness[structKey, string]{
				Backend: memory.New[structKey, string](memory.Config[string]{Shards: 4, MaxItems: 128}),
			}
		},
		kvtest.KeySet[structKey]{
			Main:    structKey{"example.com", 1},
			Other:   structKey{"example.com", 28},
			Missing: structKey{"missing", 0},
		},
		"v",
		func(a, b string) bool { return a == b },
	)
}

func TestContractStructKeyWithCustomHash(t *testing.T) {
	kvtest.Contract[structKey, string](t,
		func() kvtest.Harness[structKey, string] {
			return kvtest.Harness[structKey, string]{
				Backend: memory.New[structKey, string](
					memory.Config[string]{Shards: 4, MaxItems: 128},
					memory.WithHash(structKeyHash),
				),
			}
		},
		kvtest.KeySet[structKey]{
			Main:    structKey{"example.com", 1},
			Other:   structKey{"example.com", 28},
			Missing: structKey{"missing", 0},
		},
		"v",
		func(a, b string) bool { return a == b },
	)
}

func TestCustomHashUsed(t *testing.T) {
	var calls atomic.Int64
	counter := func(k string) uint64 {
		calls.Add(1)
		return fnvString(k)
	}

	b := memory.New[string, string](
		memory.Config[string]{Shards: 1, MaxItems: 10},
		memory.WithHash[string](counter),
	)

	ctx := context.Background()
	if err := b.Save(ctx, "k", "v", time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Load(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if err := b.Drop(ctx, "k"); err != nil {
		t.Fatal(err)
	}

	if got := calls.Load(); got < 3 {
		t.Fatalf("custom hash not invoked for all operations: calls=%d", got)
	}
}

func TestWithHashNilIsNoop(t *testing.T) {
	// Passing a nil function must not disable sharding; the default
	// hash.Sum should remain in effect.
	b := memory.New[string, string](
		memory.Config[string]{Shards: 1, MaxItems: 10},
		memory.WithHash[string](nil),
	)
	ctx := context.Background()
	if err := b.Save(ctx, "k", "v", time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Load(ctx, "k"); err != nil {
		t.Fatalf("default hash must remain after nil WithHash: %v", err)
	}
}

func TestByteBudgetEviction(t *testing.T) {
	b := memory.New[string, string](memory.Config[string]{
		Shards:   1,
		MaxItems: 1000,
		MaxBytes: 100,
		SizeOf:   func(s string) int { return len(s) },
	})
	ctx := context.Background()
	payload := string(make([]byte, 30))
	for i := 0; i < 10; i++ {
		_ = b.Save(ctx, fmt.Sprintf("k%d", i), payload, time.Minute)
	}
	remaining := 0
	for i := 0; i < 10; i++ {
		if _, err := b.Load(ctx, fmt.Sprintf("k%d", i)); err == nil {
			remaining++
		}
	}
	if remaining > 4 {
		t.Fatalf("byte budget not enforced: %d entries remain", remaining)
	}
}

func TestLazyExpiry(t *testing.T) {
	b := memory.New[string, string](memory.Config[string]{Shards: 1, MaxItems: 10})
	ctx := context.Background()
	_ = b.Save(ctx, "k", "v", 20*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	if _, err := b.Load(ctx, "k"); !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("want miss after TTL, got %v", err)
	}
}

func TestConcurrentDifferentKeys(t *testing.T) {
	b := memory.New[string, int](memory.Config[int]{Shards: 16, MaxItems: 100000})
	ctx := context.Background()
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				key := fmt.Sprintf("g%d-k%d", g, i)
				_ = b.Save(ctx, key, i, time.Minute)
			}
		}(g)
	}
	wg.Wait()
	for g := 0; g < 16; g++ {
		for i := 0; i < 500; i++ {
			key := fmt.Sprintf("g%d-k%d", g, i)
			if _, err := b.Load(ctx, key); err != nil {
				t.Fatalf("lost key %s: %v", key, err)
			}
		}
	}
}

// fnvString mirrors the helper used by hash.Sum for strings. Kept local to
// avoid exporting internals from the hash package.
func fnvString(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}
