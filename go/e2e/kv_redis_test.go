//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRedisKV_CRUD_And_TTL(t *testing.T) {
	flushRedis(t)
	store := createTestStore(t)
	ctx := context.Background()

	// 1. Set & Get
	err := store.KV().Set(ctx, "node:1:status", []byte("active"), nil)
	if err != nil {
		t.Fatalf("KV.Set failed: %v", err)
	}

	val, exists, err := store.KV().Get(ctx, "node:1:status")
	if err != nil || !exists {
		t.Fatalf("KV.Get failed: exists=%v, err=%v", exists, err)
	}
	if string(val) != "active" {
		t.Fatalf("expected 'active', got '%s'", string(val))
	}

	// 2. Exists
	ex, err := store.KV().Exists(ctx, "node:1:status")
	if err != nil || !ex {
		t.Fatalf("KV.Exists failed: %v", err)
	}

	// 3. SetIfNotExists
	setNX, err := store.KV().SetIfNotExists(ctx, "node:1:status", []byte("duplicate"), nil)
	if err != nil || setNX {
		t.Fatalf("expected SetIfNotExists to be false for existing key, got %v, err=%v", setNX, err)
	}

	setNXNew, err := store.KV().SetIfNotExists(ctx, "node:2:status", []byte("standby"), nil)
	if err != nil || !setNXNew {
		t.Fatalf("expected SetIfNotExists to be true for new key, got %v, err=%v", setNXNew, err)
	}

	// 4. TTL expiration
	ttl := 300 * time.Millisecond
	err = store.KV().Set(ctx, "ephemeral:key", []byte("temp"), &ttl)
	if err != nil {
		t.Fatalf("KV.Set with TTL failed: %v", err)
	}

	time.Sleep(400 * time.Millisecond)
	_, exists, err = store.KV().Get(ctx, "ephemeral:key")
	if err != nil || exists {
		t.Fatalf("expected key to expire, but exists=%v, err=%v", exists, err)
	}

	// 5. Delete
	err = store.KV().Delete(ctx, "node:1:status")
	if err != nil {
		t.Fatalf("KV.Delete failed: %v", err)
	}

	ex, err = store.KV().Exists(ctx, "node:1:status")
	if err != nil || ex {
		t.Fatalf("expected key to be deleted, but exists=%v", ex)
	}
}

func TestRedisKV_Concurrency(t *testing.T) {
	flushRedis(t)
	store := createTestStore(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("concurrent:key:%d", idx)
			val := fmt.Sprintf("val-%d", idx)

			if err := store.KV().Set(ctx, key, []byte(val), nil); err != nil {
				t.Errorf("Set failed for %s: %v", key, err)
				return
			}

			readBack, ok, err := store.KV().Get(ctx, key)
			if err != nil || !ok || string(readBack) != val {
				t.Errorf("Get failed for %s: ok=%v, got=%s, err=%v", key, ok, string(readBack), err)
			}
		}(i)
	}
	wg.Wait()
}
