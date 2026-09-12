package dummy_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/avalak/kv"
	"github.com/avalak/kv/dummy"
)

func TestAlwaysMisses(t *testing.T) {
	b := dummy.New[string, int]()
	ctx := context.Background()
	_ = b.Save(ctx, "k", 42, time.Hour)
	if _, err := b.Load(ctx, "k"); !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("dummy must always miss, got %v", err)
	}
}

func TestSaveIsNoop(t *testing.T) {
	b := dummy.New[string, int]()
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		if err := b.Save(ctx, "k", i, time.Minute); err != nil {
			t.Fatalf("Save must never error: %v", err)
		}
	}
}

func TestLoadReturnsZeroValue(t *testing.T) {
	type payload struct{ X int }
	b := dummy.New[string, *payload]()
	got, err := b.Load(context.Background(), "k")
	if !errors.Is(err, kv.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if got != nil {
		t.Fatalf("want nil, got %v", got)
	}
}

func TestDropNeverErrors(t *testing.T) {
	b := dummy.New[string, int]()
	if err := b.Drop(context.Background(), "anything"); err != nil {
		t.Fatalf("Drop must not error: %v", err)
	}
}

func TestCloseNeverErrors(t *testing.T) {
	b := dummy.New[string, int]()
	if err := b.Close(); err != nil {
		t.Fatalf("Close must not error: %v", err)
	}
}
