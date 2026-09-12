// Package hash provides a stable 64-bit hash for keys used by sharded
// in-memory structures. Fast paths cover strings, integers and fixed-size
// byte arrays; other types fall back to fmt.Sprint which allocates.
package hash

import (
	"fmt"
	"hash/fnv"
)

// Sum returns a 64-bit hash for any comparable value.
//
// Fast, allocation-free paths cover:
//   - string and named string types
//   - signed and unsigned integers up to 64 bits
//   - [16]byte (UUID) and [32]byte (hashes)
//
// All other types, including structs, fall back to fmt.Sprint followed by
// FNV-1a. The fallback allocates on every call and has two known issues:
//
//   - structs with string fields containing spaces can collide:
//     {A: "a b", B: "c"} and {A: "a", B: "b c"} both stringify to
//     "{a b c}" and hash identically;
//   - different struct types with identical field values hash identically.
//
// For struct keys on a hot path, define a custom function and pass it to
// memory.New via memory.WithHash instead of relying on this fallback.
//
// The hash is not cryptographic and not guaranteed stable across releases.
func Sum[K comparable](k K) uint64 {
	switch v := any(k).(type) {
	case string:
		return fnvString(v)
	case int:
		return mix(uint64(v))
	case int8:
		return mix(uint64(v))
	case int16:
		return mix(uint64(v))
	case int32:
		return mix(uint64(v))
	case int64:
		return mix(uint64(v))
	case uint:
		return mix(uint64(v))
	case uint8:
		return mix(uint64(v))
	case uint16:
		return mix(uint64(v))
	case uint32:
		return mix(uint64(v))
	case uint64:
		return mix(v)
	case [16]byte:
		return fnvBytes(v[:])
	case [32]byte:
		return fnvBytes(v[:])
	default:
		return fnvString(fmt.Sprint(k))
	}
}

func fnvString(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func fnvBytes(b []byte) uint64 {
	h := fnv.New64a()
	_, _ = h.Write(b)
	return h.Sum64()
}

// mix applies a splitmix64 finalizer to spread sequential integer keys
// across shards evenly.
func mix(x uint64) uint64 {
	x += 0x9E3779B97F4A7C15
	x = (x ^ (x >> 30)) * 0xBF58476D1CE4E5B9
	x = (x ^ (x >> 27)) * 0x94D049BB133111EB
	return x ^ (x >> 31)
}
