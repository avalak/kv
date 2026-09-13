// Package memory provides a sharded, TTL-aware LRU backend.
package memory

import (
	"context"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

	"github.com/avalak/kv"
	"github.com/avalak/kv/hash"
)

// Config configures the memory backend. Shards is rounded up to the next
// power of two; zero selects 32. MaxItems and MaxBytes are divided evenly
// across shards.
type Config[V any] struct {
	Shards   int
	MaxItems int
	MaxBytes int64
	SizeOf   func(V) int
}

// Option customises a Backend at construction time.
type Option[K comparable] func(*options[K])

type options[K comparable] struct {
	hash func(K) uint64
}

// WithHash overrides the default hash function used to spread keys across
// shards. Use it when K is a struct or another type for which hash.Sum's
// fmt.Sprint fallback is not appropriate (allocates, collides on string
// fields containing spaces).
//
// The function must be pure, deterministic and safe for concurrent calls.
// Passing nil is a no-op.
func WithHash[K comparable](h func(K) uint64) Option[K] {
	return func(o *options[K]) {
		if h != nil {
			o.hash = h
		}
	}
}

// Backend is a sharded in-memory cache.
type Backend[K comparable, V any] struct {
	shards           []*shard[K, V]
	mask             uint64
	cfg              Config[V]
	hash             func(K) uint64
	perShardMaxBytes int64 // 0 disables the byte budget
}

type shard[K comparable, V any] struct {
	mu    sync.Mutex
	lru   *lru.Cache[K, item[V]]
	bytes int64
}

type item[V any] struct {
	v         V
	expiresAt time.Time
}

// New builds a sharded memory backend. Without options it uses hash.Sum as
// the sharding function.
func New[K comparable, V any](cfg Config[V], opts ...Option[K]) *Backend[K, V] {
	o := options[K]{hash: hash.Sum[K]}
	for _, opt := range opts {
		opt(&o)
	}

	n := cfg.Shards
	if n <= 0 {
		n = 32
	} else {
		n = nextPow2(n)
	}

	maxItems := cfg.MaxItems / n
	if maxItems < 1 {
		maxItems = 1
	}

	// Distribute the byte budget across shards. Round up so that a
	// MaxBytes smaller than the shard count does not silently disable
	// eviction.
	perShardBytes := int64(0)
	if cfg.MaxBytes > 0 {
		perShardBytes = cfg.MaxBytes / int64(n)
		if perShardBytes < 1 {
			perShardBytes = 1
		}
	}

	b := &Backend[K, V]{
		shards:           make([]*shard[K, V], n),
		mask:             uint64(n - 1),
		cfg:              cfg,
		hash:             o.hash,
		perShardMaxBytes: perShardBytes,
	}
	for i := range b.shards {
		c, _ := lru.New[K, item[V]](maxItems)
		b.shards[i] = &shard[K, V]{lru: c}
	}
	return b
}

func (b *Backend[K, V]) Load(ctx context.Context, key K) (V, error) {
	if err := ctx.Err(); err != nil {
		var zero V
		return zero, err
	}
	s := b.shard(key)
	it, ok := s.lru.Get(key) // internally synchronised
	if !ok {
		var zero V
		return zero, kv.ErrNotFound
	}
	// Lazy purge; re-check under lock against concurrent Save.
	if time.Now().After(it.expiresAt) {
		s.mu.Lock()
		if cur, ok := s.lru.Peek(key); ok && time.Now().After(cur.expiresAt) {
			s.lru.Remove(key)
			s.bytes -= b.size(cur.v)
		}
		s.mu.Unlock()
		var zero V
		return zero, kv.ErrNotFound
	}
	return it.v, nil
}

func (b *Backend[K, V]) Save(ctx context.Context, key K, v V, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if ttl <= 0 {
		return nil
	}
	s := b.shard(key)
	size := b.size(v)

	s.mu.Lock()
	defer s.mu.Unlock()

	if old, ok := s.lru.Peek(key); ok {
		s.bytes -= b.size(old.v)
	}
	s.lru.Add(key, item[V]{v: v, expiresAt: time.Now().Add(ttl)})
	s.bytes += size

	for b.perShardMaxBytes > 0 && s.bytes > b.perShardMaxBytes {
		_, evicted, ok := s.lru.RemoveOldest()
		if !ok {
			break
		}
		s.bytes -= b.size(evicted.v)
	}
	return nil
}

func (b *Backend[K, V]) Drop(ctx context.Context, key K) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s := b.shard(key)
	s.mu.Lock()
	if old, ok := s.lru.Peek(key); ok {
		s.bytes -= b.size(old.v)
	}
	s.lru.Remove(key)
	s.mu.Unlock()
	return nil
}

// Has reports whether key is present and unexpired. Peek does not update
// recency, so Has is an observation, not an access.
func (b *Backend[K, V]) Has(ctx context.Context, key K) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	s := b.shard(key)
	it, ok := s.lru.Peek(key)
	if !ok {
		return false, nil
	}
	// Lazy purge, same pattern as Load: re-check under lock in case Save
	// replaced the entry concurrently.
	if time.Now().After(it.expiresAt) {
		s.mu.Lock()
		if cur, ok := s.lru.Peek(key); ok && time.Now().After(cur.expiresAt) {
			s.lru.Remove(key)
			s.bytes -= b.size(cur.v)
		}
		s.mu.Unlock()
		return false, nil
	}
	return true, nil
}

func (b *Backend[K, V]) Close() error { return nil }

func (b *Backend[K, V]) size(v V) int64 {
	if b.cfg.SizeOf == nil {
		return 1
	}
	return int64(b.cfg.SizeOf(v))
}

func (b *Backend[K, V]) shard(key K) *shard[K, V] {
	return b.shards[b.hash(key)&b.mask]
}

func nextPow2(n int) int {
	if n <= 1 {
		return 1
	}
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}
