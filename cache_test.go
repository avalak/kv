package kv_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/avalak/kv"
	"github.com/avalak/kv/dummy"
	"github.com/avalak/kv/memory"
)

type payload struct {
	Data []byte
}

func clonePayload(p *payload) *payload {
	return &payload{Data: append([]byte(nil), p.Data...)}
}

func newTestCache(t *testing.T, clone func(*payload) *payload) *kv.Cache[string, *payload] {
	t.Helper()
	backend := memory.New[string, *payload](memory.Config[*payload]{
		Shards:   4,
		MaxItems: 128,
		SizeOf:   func(p *payload) int { return len(p.Data) },
	})
	return kv.New[string, *payload](backend, clone)
}

func TestCacheMiss(t *testing.T) {
	c := newTestCache(t, clonePayload)
	_, ok, err := c.Get(context.Background(), "missing")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("want miss")
	}
}

func TestCacheRoundTrip(t *testing.T) {
	c := newTestCache(t, clonePayload)
	ctx := context.Background()
	want := &payload{Data: []byte("hello")}
	if err := c.Put(ctx, "k", want, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, ok, err := c.Get(ctx, "k")
	if err != nil || !ok {
		t.Fatalf("want hit, got ok=%v err=%v", ok, err)
	}
	if string(got.Data) != "hello" {
		t.Fatalf("got %q", got.Data)
	}
}

func TestCacheClonesOnPut(t *testing.T) {
	c := newTestCache(t, clonePayload)
	ctx := context.Background()
	orig := &payload{Data: []byte("abc")}
	_ = c.Put(ctx, "k", orig, time.Minute)
	orig.Data[0] = 'X'
	got, _, _ := c.Get(ctx, "k")
	if got.Data[0] != 'a' {
		t.Fatalf("mutating original leaked into cache: %q", got.Data)
	}
}

func TestCacheClonesOnGet(t *testing.T) {
	c := newTestCache(t, clonePayload)
	ctx := context.Background()
	_ = c.Put(ctx, "k", &payload{Data: []byte("abc")}, time.Minute)
	first, _, _ := c.Get(ctx, "k")
	first.Data[0] = 'X'
	second, _, _ := c.Get(ctx, "k")
	if second.Data[0] != 'a' {
		t.Fatalf("mutating returned value leaked into cache: %q", second.Data)
	}
}

func TestCacheNoClone(t *testing.T) {
	c := newTestCache(t, nil)
	ctx := context.Background()
	orig := &payload{Data: []byte("abc")}
	_ = c.Put(ctx, "k", orig, time.Minute)
	orig.Data[0] = 'X'
	got, _, _ := c.Get(ctx, "k")
	if got.Data[0] != 'X' {
		t.Fatalf("nil clone should share reference, got %q", got.Data)
	}
}

func TestCacheZeroTTLNoop(t *testing.T) {
	c := newTestCache(t, clonePayload)
	ctx := context.Background()
	_ = c.Put(ctx, "k", &payload{Data: []byte("x")}, 0)
	if _, ok, _ := c.Get(ctx, "k"); ok {
		t.Fatal("zero TTL must not store")
	}
}

func TestCacheDelete(t *testing.T) {
	c := newTestCache(t, clonePayload)
	ctx := context.Background()
	_ = c.Put(ctx, "k", &payload{Data: []byte("x")}, time.Minute)
	if err := c.Delete(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := c.Get(ctx, "k"); ok {
		t.Fatal("want miss after Delete")
	}
}

func TestCacheWithDummy(t *testing.T) {
	c := kv.New[string, *payload](dummy.New[string, *payload](), clonePayload)
	ctx := context.Background()
	_ = c.Put(ctx, "k", &payload{Data: []byte("x")}, time.Minute)
	if _, ok, _ := c.Get(ctx, "k"); ok {
		t.Fatal("dummy backend must always miss")
	}
}

func TestCacheNilBackendPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("want panic on nil backend")
		}
	}()
	kv.New[string, *payload](nil, nil)
}

func TestCacheConcurrent(t *testing.T) {
	c := newTestCache(t, clonePayload)
	ctx := context.Background()
	var ops atomic.Int64
	done := make(chan struct{})
	for i := 0; i < 16; i++ {
		go func(i int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 200; j++ {
				key := string(rune('a' + i))
				_ = c.Put(ctx, key, &payload{Data: []byte{byte(j)}}, time.Minute)
				_, _, _ = c.Get(ctx, key)
				ops.Add(1)
			}
		}(i)
	}
	for i := 0; i < 16; i++ {
		<-done
	}
	if ops.Load() != 3200 {
		t.Fatalf("lost operations: %d", ops.Load())
	}
}
