//go:build e2e

package e2e

import (
	"context"
	"testing"

	goystore "github.com/goy-co/goy-store/go"
)

func TestHealthCheck_BackendsHealthy(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// 1. Individual contracts
	kvH, err := store.KV().IsHealthy(ctx)
	if err != nil || kvH.State != goystore.HealthHealthy || kvH.Backend != "redis" {
		t.Fatalf("KV health failed: %+v, err=%v", kvH, err)
	}

	relH, err := store.Relational().IsHealthy(ctx)
	if err != nil || relH.State != goystore.HealthHealthy || relH.Backend != "postgres" {
		t.Fatalf("Relational health failed: %+v, err=%v", relH, err)
	}

	ssH, err := store.SortedSet().IsHealthy(ctx)
	if err != nil || ssH.State != goystore.HealthHealthy || ssH.Backend != "redis" {
		t.Fatalf("SortedSet health failed: %+v, err=%v", ssH, err)
	}

	psH, err := store.PubSub().IsHealthy(ctx)
	if err != nil || psH.State != goystore.HealthHealthy || psH.Backend != "redis" {
		t.Fatalf("PubSub health failed: %+v, err=%v", psH, err)
	}

	blobH, err := store.Blob().IsHealthy(ctx)
	if err != nil || blobH.State != goystore.HealthHealthy || blobH.Backend != "local" {
		t.Fatalf("Blob health failed: %+v, err=%v", blobH, err)
	}

	// 2. Consolidated health
	health := store.HealthCheck(ctx)
	if health.State != goystore.HealthHealthy {
		t.Fatalf("Consolidated health expected healthy, got %s", health.State)
	}
	if len(health.Contracts) != 5 {
		t.Fatalf("expected 5 contracts, got %d", len(health.Contracts))
	}
}
