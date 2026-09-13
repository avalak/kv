package utils

import (
	"runtime"
	"testing"
)

func TestNextPow2(t *testing.T) {
	cases := []struct{ n, want int }{
		{-1, 1},
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{7, 8},
		{8, 8},
		{9, 16},
		{16, 16},
		{17, 32},
		{31, 32},
		{32, 32},
		{33, 64},
		{1024, 1024},
		{1025, 2048},
	}
	for _, c := range cases {
		if got := NextPow2(c.n); got != c.want {
			t.Errorf("NextPow2(%d) = %d, want %d", c.n, got, c.want)
		}
	}
}

func TestParallelism(t *testing.T) {
	got := Parallelism()
	if got < 1 || got&(got-1) != 0 {
		t.Fatalf("Parallelism() = %d, not a power of two >= 1", got)
	}
	want := NextPow2(runtime.NumCPU())
	if got != want {
		t.Fatalf("Parallelism() = %d, want NextPow2(NumCPU) = %d", got, want)
	}
}
