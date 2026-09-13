// Package utils provides small helpers shared across kv backends.
package utils

import (
	"math/bits"
	"runtime"
)

// Parallelism returns the smallest power of two >= runtime.NumCPU(),
// with a minimum of 1.
//
// Used as the default shard count for in-memory caches: enough shards to
// saturate available cores under contention, rounded up so shard masking
// works with bitwise AND.
func Parallelism() int {
	return NextPow2(runtime.NumCPU())
}

// NextPow2 returns the smallest power of two >= n, with a minimum of 1.
// Returns 1 for n <= 1.
func NextPow2(n int) int {
	if n <= 1 {
		return 1
	}
	return 1 << bits.Len(uint(n-1))
}
