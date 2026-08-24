package goystore

import (
	"context"
	"os"
	"strings"
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

func TestMemorySortedSetStore(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	ss := store.SortedSet()

	// Test Add
	if err := ss.Add(ctx, "nodes", "node-1", 100.0); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := ss.Add(ctx, "nodes", "node-2", 200.0); err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if err := ss.Add(ctx, "nodes", "node-3", 150.0); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Test Count
	count, err := ss.Count(ctx, "nodes")
	if err != nil || count != 3 {
		t.Fatalf("Count returned (%d, %v), expected (3, nil)", count, err)
	}

	// Test Score
	score, err := ss.Score(ctx, "nodes", "node-2")
	if err != nil || score == nil || *score != 200.0 {
		t.Fatalf("Score returned (%v, %v), expected (200.0, nil)", score, err)
	}

	scoreNone, err := ss.Score(ctx, "nodes", "unknown-node")
	if err != nil || scoreNone != nil {
		t.Fatalf("Score for unknown node returned (%v, %v), expected (nil, nil)", scoreNone, err)
	}

	// Test RangeByScore
	rangeMembers, err := ss.RangeByScore(ctx, "nodes", 100.0, 180.0, nil)
	if err != nil {
		t.Fatalf("RangeByScore failed: %v", err)
	}
	if len(rangeMembers) != 2 {
		t.Fatalf("expected 2 members, got %d", len(rangeMembers))
	}
	if rangeMembers[0].Member != "node-1" || rangeMembers[0].Score != 100.0 {
		t.Fatalf("expected node-1 at 100.0, got %+v", rangeMembers[0])
	}
	if rangeMembers[1].Member != "node-3" || rangeMembers[1].Score != 150.0 {
		t.Fatalf("expected node-3 at 150.0, got %+v", rangeMembers[1])
	}

	// Test RemoveRange
	removed, err := ss.RemoveRange(ctx, "nodes", 100.0, 150.0)
	if err != nil || removed != 2 {
		t.Fatalf("RemoveRange returned (%d, %v), expected (2, nil)", removed, err)
	}

	count, _ = ss.Count(ctx, "nodes")
	if count != 1 {
		t.Fatalf("expected 1 remaining node, got %d", count)
	}
}

func TestRedisSortedSetStore_Config(t *testing.T) {
	cfg := &Config{
		KV: KVConfig{
			Backend: "redis",
			URL:     "redis://127.0.0.1:6379",
		},
		SortedSet: SortedSetConfig{
			Backend: "redis",
		},
	}

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	if store == nil {
		t.Fatalf("expected non-nil GoyStore")
	}
	if store.SortedSet() == nil {
		t.Fatalf("expected non-nil SortedSet")
	}
}

func TestLocalBlobStore(t *testing.T) {
	ctx := context.Background()
	tempDir, err := os.MkdirTemp("", "goy_blob_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store := NewLocalBlobStore(tempDir)

	meta := &Metadata{
		ContentType: "application/json",
		Custom: map[string]string{
			"env": "testing",
		},
	}

	key := "certs/node-1.crt"
	data := []byte("CERT_PAYLOAD_TEST")

	// Put
	if err := store.Put(ctx, key, data, meta); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Get
	gotData, gotMeta, exists, err := store.Get(ctx, key)
	if err != nil || !exists {
		t.Fatalf("Get returned (%v, %v), expected exists=true, err=nil", exists, err)
	}
	if string(gotData) != string(data) {
		t.Fatalf("expected data %s, got %s", string(data), string(gotData))
	}
	if gotMeta == nil || gotMeta.ContentType != "application/json" || gotMeta.Custom["env"] != "testing" {
		t.Fatalf("unexpected metadata: %+v", gotMeta)
	}

	// List
	list, err := store.List(ctx, nil)
	if err != nil || len(list) != 1 || list[0] != key {
		t.Fatalf("List returned (%+v, %v), expected [%s]", list, err, key)
	}

	// PresignURL
	url, err := store.PresignURL(ctx, key, time.Minute)
	if err != nil || !strings.HasPrefix(url, "file://") {
		t.Fatalf("PresignURL returned (%s, %v)", url, err)
	}

	// Delete
	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, _, exists, err = store.Get(ctx, key)
	if err != nil || exists {
		t.Fatalf("expected key to be deleted")
	}

	// Traversal safety
	err = store.Put(ctx, "../secret.txt", []byte("bad"), nil)
	if err == nil {
		t.Fatalf("expected directory traversal to fail")
	}
}

func TestMemoryPubSubStore(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	store := NewMemoryStore()
	ps := store.PubSub()

	ch, err := ps.Subscribe(ctx, []string{"notifications"})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	payload := []byte("hello-goy")
	if err := ps.Publish(ctx, "notifications", payload); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	select {
	case msg := <-ch:
		if msg.Channel != "notifications" || string(msg.Payload) != string(payload) {
			t.Fatalf("unexpected message: %+v", msg)
		}
	case <-ctx.Done():
		t.Fatalf("timed out waiting for message")
	}

	if err := ps.Unsubscribe(ctx, []string{"notifications"}); err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}
}

func TestRedisPubSubStore_Config(t *testing.T) {
	cfg := &Config{
		KV: KVConfig{
			Backend: "redis",
			URL:     "redis://127.0.0.1:6379",
		},
		PubSub: PubSubConfig{
			Backend: "redis",
		},
	}

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	if store == nil || store.PubSub() == nil {
		t.Fatalf("expected non-nil PubSub store")
	}
}
