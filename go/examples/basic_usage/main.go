package main

import (
	"context"
	"fmt"
	"time"

	goystore "github.com/goy-co/goy-store/go"
)

func main() {
	fmt.Println("🚀 Starting Goy Store Basic Usage Example (Go)")

	ctx := context.Background()

	// 1. Initialize GoyStore with default in-memory configuration
	cfg := goystore.DefaultConfig()
	store, err := goystore.NewStore(cfg)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize store: %v", err))
	}

	// 2. KV Store Operations
	fmt.Println("\n📦 [KV Store]")
	ttl := 60 * time.Second
	if err := store.KV().Set(ctx, "node:1:status", []byte("active"), &ttl); err != nil {
		panic(err)
	}

	val, exists, err := store.KV().Get(ctx, "node:1:status")
	if err != nil || !exists {
		panic(fmt.Sprintf("key not found: %v", err))
	}
	fmt.Printf("  -> Fetched key 'node:1:status': %s\n", string(val))

	// 3. Sorted Set Operations
	fmt.Println("\n⏱️ [Sorted Set Store]")
	_ = store.SortedSet().Add(ctx, "nodes:heartbeat", "node-1", 1000.0)
	_ = store.SortedSet().Add(ctx, "nodes:heartbeat", "node-2", 1050.0)

	activeNodes, err := store.SortedSet().RangeByScore(ctx, "nodes:heartbeat", 900.0, 1100.0, nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("  -> Active nodes count: %d\n", len(activeNodes))
	for _, node := range activeNodes {
		fmt.Printf("     - %s (score: %.1f)\n", node.Member, node.Score)
	}

	// 4. Blob Store Operations
	fmt.Println("\n📁 [Blob Store]")
	_ = store.Blob().Put(ctx, "configs/cluster.json", []byte(`{"cluster": "goy-alpha"}`), nil)
	presignedURL, _ := store.Blob().PresignURL(ctx, "configs/cluster.json", 5*time.Minute)
	fmt.Printf("  -> Presigned URL: %s\n", presignedURL)

	// 5. Health Check
	fmt.Println("\n🩺 [Health Check]")
	health := store.HealthCheck(ctx)
	fmt.Printf("  -> Overall Status: %s\n", health.State)
	for contract, status := range health.Contracts {
		fmt.Printf("     - %s: %s (backend: %s, latency: %dms)\n", contract, status.State, status.Backend, status.LatencyMS)
	}

	fmt.Println("\n✅ Example completed successfully.")
}
