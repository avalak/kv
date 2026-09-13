// Package dummy provides a no-op backend. Every Load misses; every Save is
// discarded. Useful for tests and for --no-cache modes.
package dummy

import (
	"context"
	"time"

	"github.com/avalak/kv"
)

type Backend[K comparable, V any] struct{}

func New[K comparable, V any]() *Backend[K, V] { return &Backend[K, V]{} }

func (*Backend[K, V]) Load(context.Context, K) (V, error) {
	var zero V
	return zero, kv.ErrNotFound
}

func (*Backend[K, V]) Save(context.Context, K, V, time.Duration) error { return nil }
func (*Backend[K, V]) Drop(context.Context, K) error                   { return nil }
func (*Backend[K, V]) Has(context.Context, K) (bool, error)            { return false, nil }
func (*Backend[K, V]) Close() error                                    { return nil }
