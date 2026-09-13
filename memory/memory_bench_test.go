package memory_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/avalak/kv/memory"
)

const (
	benchPayloadSize = 256
	benchKeyCount    = 10_000
)

// Pre-generated keys avoid strconv.Itoa allocations inside the benchmark
// loop, which would otherwise pollute B/op and allocs/op.
var benchKeys = func() []string {
	keys := make([]string, benchKeyCount)
	for i := range keys {
		keys[i] = "key-" + strconv.Itoa(i)
	}
	return keys
}()

func benchBackend(b *testing.B, shards, maxItems int) *memory.Backend[string, []byte] {
	b.Helper()
	return memory.New[string, []byte](memory.Config[[]byte]{
		Shards:   shards,
		MaxItems: maxItems,
		MaxBytes: 256 << 20,
		SizeOf:   func(v []byte) int { return len(v) },
	})
}

// --- Hot path ---

func BenchmarkLoad_Hot(b *testing.B) {
	backend := benchBackend(b, 32, 10_000)
	ctx := context.Background()
	payload := make([]byte, benchPayloadSize)
	_ = backend.Save(ctx, "hot", payload, time.Hour)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = backend.Load(ctx, "hot")
	}
}

func BenchmarkLoad_Miss(b *testing.B) {
	backend := benchBackend(b, 32, 10_000)
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = backend.Load(ctx, "absent")
	}
}

func BenchmarkSave_Hot(b *testing.B) {
	backend := benchBackend(b, 32, 10_000)
	ctx := context.Background()
	payload := make([]byte, benchPayloadSize)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = backend.Save(ctx, "hot", payload, time.Hour)
	}
}

func BenchmarkHas_Hot(b *testing.B) {
	backend := benchBackend(b, 32, 10_000)
	ctx := context.Background()
	payload := make([]byte, benchPayloadSize)
	_ = backend.Save(ctx, "hot", payload, time.Hour)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = backend.Has(ctx, "hot")
	}
}

// --- Single-key contention ---
//
// All goroutines hit one key, hence one shard. Measures the upper bound of
// damage a single hot key can do to parallelism, not backend throughput.

func BenchmarkLoad_HotKeyContention(b *testing.B) {
	backend := benchBackend(b, 32, 10_000)
	ctx := context.Background()
	payload := make([]byte, benchPayloadSize)
	_ = backend.Save(ctx, "hot", payload, time.Hour)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = backend.Load(ctx, "hot")
		}
	})
}

func BenchmarkSave_HotKeyContention(b *testing.B) {
	backend := benchBackend(b, 32, 10_000)
	ctx := context.Background()
	payload := make([]byte, benchPayloadSize)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = backend.Save(ctx, "hot", payload, time.Hour)
		}
	})
}

// --- Parallel throughput ---
//
// Keys spread across shards, so contention is on many RWMutexes rather
// than one. This is the realistic multi-core scenario.

func BenchmarkLoad_Parallel(b *testing.B) {
	backend := benchBackend(b, 32, 100_000)
	ctx := context.Background()
	payload := make([]byte, benchPayloadSize)
	for _, k := range benchKeys {
		_ = backend.Save(ctx, k, payload, time.Hour)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, _ = backend.Load(ctx, benchKeys[i%len(benchKeys)])
			i++
		}
	})
}

// BenchmarkMixed simulates a 9:1 read/write workload across 10_000 keys.
func BenchmarkMixed(b *testing.B) {
	backend := benchBackend(b, 32, 100_000)
	ctx := context.Background()
	payload := make([]byte, benchPayloadSize)
	for _, k := range benchKeys {
		_ = backend.Save(ctx, k, payload, time.Hour)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := benchKeys[i%len(benchKeys)]
			if i%10 == 0 {
				_ = backend.Save(ctx, key, payload, time.Hour)
			} else {
				_, _ = backend.Load(ctx, key)
			}
			i++
		}
	})
}

func BenchmarkShardContention(b *testing.B) {
	for _, shards := range []int{1, 4, 16, 32, 64} {
		b.Run("shards="+strconv.Itoa(shards), func(b *testing.B) {
			backend := benchBackend(b, shards, 100_000)
			ctx := context.Background()
			payload := make([]byte, benchPayloadSize)
			for _, k := range benchKeys {
				_ = backend.Save(ctx, k, payload, time.Hour)
			}

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					_, _ = backend.Load(ctx, benchKeys[i%len(benchKeys)])
					i++
				}
			})
		})
	}
}

// --- Expired path ---
//
// Full cycle: Save with 1ns TTL followed by Load/Has that purges. Each
// iteration allocates a list.Element in lru.Add, which is part of the
// measured cost. Derive pure purge by subtracting Save_Hot.
//
//	Load_Expired ≈ BenchmarkLoad_Expired − BenchmarkSave_Hot

func BenchmarkLoad_Expired(b *testing.B) {
	backend := benchBackend(b, 32, 10_000)
	ctx := context.Background()
	payload := make([]byte, benchPayloadSize)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = backend.Save(ctx, "expired", payload, time.Nanosecond)
		_, _ = backend.Load(ctx, "expired")
	}
}

func BenchmarkHas_Expired(b *testing.B) {
	backend := benchBackend(b, 32, 10_000)
	ctx := context.Background()
	payload := make([]byte, benchPayloadSize)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = backend.Save(ctx, "expired", payload, time.Nanosecond)
		_, _ = backend.Has(ctx, "expired")
	}
}

// --- Expired path, isolated ---
//
// Pre-populates N keys with a long TTL, lets them expire, and measures
// only Load/Has on expired entries. Isolates the ~250 ns purge cost from
// the ~1000 ns of Save + allocation + GC pressure in the benchmarks above.

func BenchmarkLoad_Expired_Isolated(b *testing.B) {
	backend := memory.New[string, []byte](memory.Config[[]byte]{
		Shards:   32,
		MaxItems: 2_000_000,
		MaxBytes: 1 << 30,
		SizeOf:   func(v []byte) int { return len(v) },
	})
	ctx := context.Background()
	payload := make([]byte, benchPayloadSize)

	keys := make([]string, b.N)
	for i := range keys {
		keys[i] = "expired-" + strconv.Itoa(i)
		_ = backend.Save(ctx, keys[i], payload, time.Hour)
	}
	time.Sleep(2 * time.Millisecond)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = backend.Load(ctx, keys[i])
	}
}

func BenchmarkHas_Expired_Isolated(b *testing.B) {
	backend := memory.New[string, []byte](memory.Config[[]byte]{
		Shards:   32,
		MaxItems: 2_000_000,
		MaxBytes: 1 << 30,
		SizeOf:   func(v []byte) int { return len(v) },
	})
	ctx := context.Background()
	payload := make([]byte, benchPayloadSize)

	keys := make([]string, b.N)
	for i := range keys {
		keys[i] = "expired-" + strconv.Itoa(i)
		_ = backend.Save(ctx, keys[i], payload, time.Hour)
	}
	time.Sleep(2 * time.Millisecond)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = backend.Has(ctx, keys[i])
	}
}
