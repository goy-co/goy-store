//go:build e2e

package e2e

import (
	"context"
	"testing"
)

func TestRedisSortedSet_HeartbeatLifecycle(t *testing.T) {
	flushRedis(t)
	store := createTestStore(t)
	ctx := context.Background()

	setName := "cluster:heartbeats"

	// 1. Add heartbeats
	if err := store.SortedSet().Add(ctx, setName, "node-1", 1000.0); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := store.SortedSet().Add(ctx, setName, "node-2", 1050.0); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := store.SortedSet().Add(ctx, setName, "node-3", 1100.0); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// 2. Count
	count, err := store.SortedSet().Count(ctx, setName)
	if err != nil || count != 3 {
		t.Fatalf("Count failed: count=%d, err=%v", count, err)
	}

	// 3. Score
	score, err := store.SortedSet().Score(ctx, setName, "node-2")
	if err != nil || score == nil || *score != 1050.0 {
		t.Fatalf("Score failed: score=%v, err=%v", score, err)
	}

	scoreMissing, err := store.SortedSet().Score(ctx, setName, "node-unknown")
	if err != nil || scoreMissing != nil {
		t.Fatalf("Score for missing failed: score=%v, err=%v", scoreMissing, err)
	}

	// 4. RangeByScore
	active, err := store.SortedSet().RangeByScore(ctx, setName, 1040.0, 1110.0, nil)
	if err != nil || len(active) != 2 {
		t.Fatalf("RangeByScore failed: len=%d, err=%v", len(active), err)
	}
	if active[0].Member != "node-2" || active[1].Member != "node-3" {
		t.Fatalf("RangeByScore unexpected members: %+v", active)
	}

	// 5. RemoveRange (nodes older than 1020)
	removed, err := store.SortedSet().RemoveRange(ctx, setName, 0.0, 1020.0)
	if err != nil || removed != 1 {
		t.Fatalf("RemoveRange failed: removed=%d, err=%v", removed, err)
	}

	remCount, _ := store.SortedSet().Count(ctx, setName)
	if remCount != 2 {
		t.Fatalf("expected 2 remaining, got %d", remCount)
	}

	// 6. Remove
	if err := store.SortedSet().Remove(ctx, setName, "node-3"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	finalCount, _ := store.SortedSet().Count(ctx, setName)
	if finalCount != 1 {
		t.Fatalf("expected 1 remaining, got %d", finalCount)
	}
}
