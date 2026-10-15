package kv

import (
	"context"
	"errors"
	"time"
)

// Backend is the storage abstraction. Implementations must be safe for
// concurrent use. ttl <= 0 means "do not store", except NoExpiry, which
// stores without an expiry. Both must be honoured.
//
// K must be comparable. Backends that cannot store Go values directly
// (redis, file) serialise K to a string via a caller-supplied encoder.
type Backend[K comparable, V any] interface {
	Load(ctx context.Context, key K) (V, error) // ErrNotFound on miss
	Save(ctx context.Context, key K, v V, ttl time.Duration) error
	Drop(ctx context.Context, key K) error
	Has(ctx context.Context, key K) (bool, error)
	Close() error
}

// PrefixBackend is an optional extension of Backend. A backend that
// implements it removes every entry whose key begins with a prefix;
// Cache.DeletePrefix dispatches to it.
type PrefixBackend[K comparable, V any] interface {
	Backend[K, V]

	// DropPrefix removes every entry whose key begins with prefix.
	// A backend whose key type cannot be matched by prefix returns
	// ErrPrefixUnsupported.
	DropPrefix(ctx context.Context, prefix K) error
}

// Cache wraps a Backend with defensive copying. The clone function is
// applied on both Save and Load so callers can never mutate the stored
// value through a returned reference.
//
// Pass clone == nil when V is immutable or when the backend already
// serialises values.
type Cache[K comparable, V any] struct {
	backend Backend[K, V]
	clone   func(V) V
}

// New builds a Cache. A nil backend panics; a nil clone disables copying.
func New[K comparable, V any](backend Backend[K, V], clone func(V) V) *Cache[K, V] {
	if backend == nil {
		panic("kv: nil backend")
	}
	return &Cache[K, V]{backend: backend, clone: clone}
}

func (c *Cache[K, V]) Get(ctx context.Context, key K) (V, bool, error) {
	v, err := c.backend.Load(ctx, key)
	if errors.Is(err, ErrNotFound) {
		var zero V
		return zero, false, nil
	}
	if err != nil {
		var zero V
		return zero, false, err
	}
	if c.clone != nil {
		v = c.clone(v)
	}
	return v, true, nil
}

func (c *Cache[K, V]) Put(ctx context.Context, key K, v V, ttl time.Duration) error {
	if ttl <= 0 && ttl != NoExpiry {
		return nil
	}
	if c.clone != nil {
		v = c.clone(v)
	}
	return c.backend.Save(ctx, key, v, ttl)
}

func (c *Cache[K, V]) Delete(ctx context.Context, key K) error {
	return c.backend.Drop(ctx, key)
}

// Has reports whether key is present and unexpired. It does not clone the
// value and never triggers an upstream fetch.
func (c *Cache[K, V]) Has(ctx context.Context, key K) (bool, error) {
	return c.backend.Has(ctx, key)
}

func (c *Cache[K, V]) Close() error { return c.backend.Close() }

// DeletePrefix removes every entry whose key begins with prefix. It
// returns ErrPrefixUnsupported when the backend does not implement
// PrefixBackend.
func (c *Cache[K, V]) DeletePrefix(ctx context.Context, prefix K) error {
	pb, ok := c.backend.(PrefixBackend[K, V])
	if !ok {
		return ErrPrefixUnsupported
	}
	return pb.DropPrefix(ctx, prefix)
}
