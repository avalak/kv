package hash_test

import (
	"testing"

	"github.com/avalak/kv/hash"
)

func BenchmarkSum_String(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = hash.Sum("example.com")
	}
}

func BenchmarkSum_Uint64(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = hash.Sum(uint64(i))
	}
}

func BenchmarkSum_UUID(b *testing.B) {
	var u [16]byte
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = hash.Sum(u)
	}
}

type benchStructKey struct {
	Domain string
	QType  uint16
}

// BenchmarkSum_StructFallback exposes the fmt.Sprint path: ~650 ns and 2
// allocations against ~14 ns for strings. Argument for WithHash on hot
// struct keys.
func BenchmarkSum_StructFallback(b *testing.B) {
	k := benchStructKey{"example.com", 1}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = hash.Sum(k)
	}
}
