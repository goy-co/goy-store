package goystore

import (
	"context"
	"testing"
	"time"
)

func TestMemoryKVStore(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	kv := store.KV()

	// Test Set and Get
	key := "test:key"
	val := []byte("test:value")
	if err := kv.Set(ctx, key, val, nil); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	got, exists, err := kv.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !exists {
		t.Fatalf("expected key to exist")
	}
	if string(got) != string(val) {
		t.Fatalf("expected %s, got %s", string(val), string(got))
	}

	// Test Exists
	exists, err = kv.Exists(ctx, key)
	if err != nil || !exists {
		t.Fatalf("Exists returned (%v, %v), expected (true, nil)", exists, err)
	}

	// Test SetIfNotExists when exists
	set, err := kv.SetIfNotExists(ctx, key, []byte("new-val"), nil)
	if err != nil || set {
		t.Fatalf("SetIfNotExists returned (%v, %v), expected (false, nil)", set, err)
	}

	// Test Delete
	if err := kv.Delete(ctx, key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Test Exists after delete
	exists, err = kv.Exists(ctx, key)
	if err != nil || exists {
		t.Fatalf("Exists returned (%v, %v), expected (false, nil)", exists, err)
	}

	// Test SetIfNotExists when not exists
	ttl := 10 * time.Second
	set, err = kv.SetIfNotExists(ctx, key, []byte("new-val"), &ttl)
	if err != nil || !set {
		t.Fatalf("SetIfNotExists returned (%v, %v), expected (true, nil)", set, err)
	}
}

func TestRedisKVStore_Config(t *testing.T) {
	cfg := KVConfig{
		Backend:   "redis",
		URL:       "redis://127.0.0.1:6379",
		PoolSize:  10,
		TimeoutMS: 500,
	}

	store, err := NewRedisKVStore(cfg)
	if err != nil {
		t.Fatalf("failed to create RedisKVStore: %v", err)
	}
	if store == nil {
		t.Fatalf("expected non-nil RedisKVStore")
	}
}

func TestPostgresRelationalStore_Config(t *testing.T) {
	cfg := RelationalConfig{
		Backend:  "postgres",
		URL:      "postgres://postgres:postgres@127.0.0.1:5432/goy?sslmode=disable",
		PoolSize: 5,
	}

	// Parsing URL and config creation test
	store, err := NewPostgresRelationalStore(context.Background(), cfg)
	// Note: will succeed creating the pool object even without active connection until first query
	if err != nil {
		t.Fatalf("failed to create PostgresRelationalStore: %v", err)
	}
	if store == nil {
		t.Fatalf("expected non-nil PostgresRelationalStore")
	}
	defer store.Close()
}
