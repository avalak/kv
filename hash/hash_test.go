// hash/hash_test.go
package hash_test

import (
	"testing"

	"github.com/avalak/kv/hash"
)

func TestStability(t *testing.T) {
	// Bind results to variables so staticcheck does not trigger
	a := hash.Sum("x")
	b := hash.Sum("x")
	if a != b {
		t.Fatalf("hash must be deterministic: got %d and %d", a, b)
	}
}

func TestDifferentStrings(t *testing.T) {
	if hash.Sum("a") == hash.Sum("b") {
		t.Fatal("different strings should hash differently")
	}
}

// TestSequentialUint64Distribution checks that 256 sequential keys cover
// every bucket when mapped onto 32 shards. With a good hash, 32 buckets
// and 256 keys, all buckets should be hit with overwhelming probability.
func TestSequentialUint64Distribution(t *testing.T) {
	const shards = 32
	mask := uint64(shards - 1)
	buckets := make([]int, shards)
	for i := uint64(0); i < 256; i++ {
		buckets[hash.Sum(i)&mask]++
	}
	for i, c := range buckets {
		if c == 0 {
			t.Fatalf("bucket %d empty for 256 sequential uint64 keys", i)
		}
	}
}

func TestStructFallsBack(t *testing.T) {
	type k struct {
		A string
		B int
	}
	if hash.Sum(k{"x", 1}) == hash.Sum(k{"x", 2}) {
		t.Fatal("struct hash should differ on different fields")
	}
}

func TestUUIDKey(t *testing.T) {
	var u1, u2 [16]byte
	u1[0] = 1
	u2[0] = 2
	if hash.Sum(u1) == hash.Sum(u2) {
		t.Fatal("different UUIDs should hash differently")
	}
}
