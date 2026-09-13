// Package kvtest provides a reusable contract test for Backend implementations.
package kvtest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/avalak/kv"
)

// Harness bundles a fresh backend with an optional clock-advance hook.
// Backends driven by a virtual clock (miniredis) set Advance; real-clock
// backends leave it nil and Contract falls back to time.Sleep.
type Harness[K comparable, V any] struct {
	Backend kv.Backend[K, V]
	Advance func(d time.Duration)
}

// KeySet supplies the keys used by the contract tests.
type KeySet[K comparable] struct {
	Main    K
	Other   K
	Missing K
}

// Contract runs the standard behavioural suite against fresh backends.
// equal compares two values for semantic equality; pass nil to skip
// value comparison.
func Contract[K comparable, V any](
	t *testing.T,
	newHarness func() Harness[K, V],
	keys KeySet[K],
	sample V,
	equal func(a, b V) bool,
) {
	t.Helper()
	ctx := context.Background()

	// newBackend returns a fresh backend and registers a cleanup that
	// closes it. Close errors are ignored: cleanup runs after the test
	// body, and a failing Close is not a contract violation.
	newBackend := func(t *testing.T) kv.Backend[K, V] {
		t.Helper()
		h := newHarness()
		t.Cleanup(func() { _ = h.Backend.Close() })
		return h.Backend
	}

	// newBackendWithAdvance is like newBackend but also exposes the clock
	// hook so TTL tests can drive virtual time.
	newBackendWithAdvance := func(t *testing.T) (kv.Backend[K, V], func(time.Duration)) {
		t.Helper()
		h := newHarness()
		t.Cleanup(func() { _ = h.Backend.Close() })
		advance := func(d time.Duration) {
			if h.Advance != nil {
				h.Advance(d)
				time.Sleep(20 * time.Millisecond)
				return
			}
			time.Sleep(d)
		}
		return h.Backend, advance
	}

	t.Run("MissOnEmpty", func(t *testing.T) {
		b := newBackend(t)
		if _, err := b.Load(ctx, keys.Missing); !errors.Is(err, kv.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})

	t.Run("SaveLoad", func(t *testing.T) {
		b := newBackend(t)
		if err := b.Save(ctx, keys.Main, sample, time.Minute); err != nil {
			t.Fatal(err)
		}
		got, err := b.Load(ctx, keys.Main)
		if err != nil {
			t.Fatal(err)
		}
		if equal != nil && !equal(got, sample) {
			t.Fatalf("value mismatch")
		}
	})

	t.Run("Expiry", func(t *testing.T) {
		b, advance := newBackendWithAdvance(t)
		if err := b.Save(ctx, keys.Main, sample, 20*time.Millisecond); err != nil {
			t.Fatal(err)
		}
		advance(60 * time.Millisecond)
		if _, err := b.Load(ctx, keys.Main); !errors.Is(err, kv.ErrNotFound) {
			t.Fatalf("want ErrNotFound after TTL, got %v", err)
		}
	})

	t.Run("ZeroTTLIsNoop", func(t *testing.T) {
		b := newBackend(t)
		if err := b.Save(ctx, keys.Main, sample, 0); err != nil {
			t.Fatal(err)
		}
		if _, err := b.Load(ctx, keys.Main); !errors.Is(err, kv.ErrNotFound) {
			t.Fatalf("zero TTL must not store, got %v", err)
		}
	})

	t.Run("NegativeTTLIsNoop", func(t *testing.T) {
		b := newBackend(t)
		if err := b.Save(ctx, keys.Main, sample, -time.Second); err != nil {
			t.Fatal(err)
		}
		if _, err := b.Load(ctx, keys.Main); !errors.Is(err, kv.ErrNotFound) {
			t.Fatalf("negative TTL must not store, got %v", err)
		}
	})

	t.Run("Drop", func(t *testing.T) {
		b := newBackend(t)
		_ = b.Save(ctx, keys.Main, sample, time.Minute)
		if err := b.Drop(ctx, keys.Main); err != nil {
			t.Fatal(err)
		}
		if _, err := b.Load(ctx, keys.Main); !errors.Is(err, kv.ErrNotFound) {
			t.Fatalf("want ErrNotFound after Drop, got %v", err)
		}
	})

	t.Run("DropMissingIsOK", func(t *testing.T) {
		b := newBackend(t)
		if err := b.Drop(ctx, keys.Missing); err != nil {
			t.Fatalf("Drop on missing key must not error: %v", err)
		}
	})

	t.Run("HasMissing", func(t *testing.T) {
		b := newBackend(t)
		ok, err := b.Has(ctx, keys.Missing)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatal("Has on missing key must return false")
		}
	})

	t.Run("HasAfterSave", func(t *testing.T) {
		b := newBackend(t)
		if err := b.Save(ctx, keys.Main, sample, time.Minute); err != nil {
			t.Fatal(err)
		}
		ok, err := b.Has(ctx, keys.Main)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatal("Has after Save must return true")
		}
	})

	t.Run("HasAfterDrop", func(t *testing.T) {
		b := newBackend(t)
		_ = b.Save(ctx, keys.Main, sample, time.Minute)
		_ = b.Drop(ctx, keys.Main)
		ok, err := b.Has(ctx, keys.Main)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatal("Has after Drop must return false")
		}
	})

	t.Run("HasExpired", func(t *testing.T) {
		b, advance := newBackendWithAdvance(t)
		if err := b.Save(ctx, keys.Main, sample, 20*time.Millisecond); err != nil {
			t.Fatal(err)
		}
		advance(60 * time.Millisecond)
		ok, err := b.Has(ctx, keys.Main)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			t.Fatal("Has on expired key must return false")
		}
	})

	t.Run("Overwrite", func(t *testing.T) {
		b := newBackend(t)
		_ = b.Save(ctx, keys.Main, sample, time.Minute)
		_ = b.Save(ctx, keys.Main, sample, time.Minute)
		if _, err := b.Load(ctx, keys.Main); err != nil {
			t.Fatalf("overwrite should succeed: %v", err)
		}
	})

	t.Run("OverwriteResetsTTL", func(t *testing.T) {
		b, advance := newBackendWithAdvance(t)
		_ = b.Save(ctx, keys.Main, sample, 20*time.Millisecond)
		advance(15 * time.Millisecond)
		_ = b.Save(ctx, keys.Main, sample, time.Minute)
		advance(20 * time.Millisecond)
		if _, err := b.Load(ctx, keys.Main); err != nil {
			t.Fatalf("second Save must refresh TTL: %v", err)
		}
	})

	t.Run("IndependentKeys", func(t *testing.T) {
		b := newBackend(t)
		_ = b.Save(ctx, keys.Main, sample, time.Minute)
		if _, err := b.Load(ctx, keys.Other); !errors.Is(err, kv.ErrNotFound) {
			t.Fatalf("key Other must not see Main's value, got %v", err)
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		b := newBackend(t)
		done := make(chan struct{})
		for i := 0; i < 8; i++ {
			go func() {
				defer func() { done <- struct{}{} }()
				for j := 0; j < 100; j++ {
					_ = b.Save(ctx, keys.Main, sample, time.Minute)
					_, _ = b.Load(ctx, keys.Main)
				}
			}()
		}
		for i := 0; i < 8; i++ {
			<-done
		}
	})
}
