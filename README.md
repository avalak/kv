# kv

Generic, TTL-aware cache with pluggable backends.

## Usage

```go
import (
    "github.com/avalak/kv"
    "github.com/avalak/kv/memory"
)

cache := kv.New[string, *Entry](
    memory.New[string, *Entry](memory.Config[*Entry]{
        Shards:   32,
        MaxItems: 10000,
        MaxBytes: 256 << 20,
        SizeOf:   func(e *Entry) int { return len(e.Body) },
    }),
    (*Entry).Clone,
)

_ = cache.Put(ctx, key, entry, 15*time.Minute)
got, ok, err := cache.Get(ctx, key)
```

Backends: `memory` (sharded LRU), `dummy` (no-op).

TTL is supplied by the caller at `Put` time; the cache never inspects the value.

## Custom keys

For struct keys on a hot path, provide a hash function:

```go
memory.New[dnsKey, *Msg](cfg, memory.WithHash(dnsHash))
```

Without it, the default hash.Sum falls back to fmt.Sprint for structs,
which allocates and can collide on string fields containing spaces.
