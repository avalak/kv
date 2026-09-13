package kv_test

import (
	"context"
	"testing"
	"time"

	"github.com/avalak/kv"
	"github.com/avalak/kv/memory"
)

func benchCache(b *testing.B, clone func(*payload) *payload) *kv.Cache[string, *payload] {
	b.Helper()
	backend := memory.New[string, *payload](memory.Config[*payload]{
		Shards:   32,
		MaxItems: 10_000,
		SizeOf:   func(p *payload) int { return len(p.Data) },
	})
	return kv.New[string, *payload](backend, clone)
}

func BenchmarkCacheGet_WithClone(b *testing.B) {
	c := benchCache(b, clonePayload)
	ctx := context.Background()
	_ = c.Put(ctx, "hot", &payload{Data: make([]byte, 1024)}, time.Hour)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = c.Get(ctx, "hot")
	}
}

func BenchmarkCacheGet_NoClone(b *testing.B) {
	c := benchCache(b, nil)
	ctx := context.Background()
	_ = c.Put(ctx, "hot", &payload{Data: make([]byte, 1024)}, time.Hour)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = c.Get(ctx, "hot")
	}
}

func BenchmarkCacheGet_SmallClone(b *testing.B) {
	c := benchCache(b, func(p *payload) *payload {
		cp := *p
		return &cp
	})
	ctx := context.Background()
	_ = c.Put(ctx, "hot", &payload{Data: []byte("x")}, time.Hour)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = c.Get(ctx, "hot")
	}
}

func BenchmarkCacheHas(b *testing.B) {
	c := benchCache(b, clonePayload)
	ctx := context.Background()
	_ = c.Put(ctx, "hot", &payload{Data: make([]byte, 1024)}, time.Hour)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Has(ctx, "hot")
	}
}

// BenchmarkCacheGet_Parallel hits a single key: measures contention, not
// throughput. See BenchmarkLoad_Parallel in the memory package for the
// realistic multi-key case.
func BenchmarkCacheGet_Parallel(b *testing.B) {
	c := benchCache(b, clonePayload)
	ctx := context.Background()
	_ = c.Put(ctx, "hot", &payload{Data: make([]byte, 1024)}, time.Hour)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, _ = c.Get(ctx, "hot")
		}
	})
}

func BenchmarkCachePut_WithClone(b *testing.B) {
	c := benchCache(b, clonePayload)
	ctx := context.Background()
	p := &payload{Data: make([]byte, 1024)}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Put(ctx, "hot", p, time.Hour)
	}
}
