package goystore

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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

func TestMetricsTracking(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := RegisterMetrics(reg)

	cfg := DefaultConfig()
	store, err := NewStoreWithMetrics(cfg, metrics)
	if err != nil {
		t.Fatalf("NewStoreWithMetrics failed: %v", err)
	}

	ctx := context.Background()

	// Perform KV ops
	_ = store.KV().Set(ctx, "metrics:key", []byte("1"), nil)
	_, _, _ = store.KV().Get(ctx, "metrics:key")

	// Perform SortedSet ops
	_ = store.SortedSet().Add(ctx, "metrics:set", "m1", 1.0)
	_, _ = store.SortedSet().Score(ctx, "metrics:set", "m1")

	// Perform Blob ops
	_ = store.Blob().Put(ctx, "metrics/file.txt", []byte("data"), nil)
	_, _, _, _ = store.Blob().Get(ctx, "metrics/file.txt")

	// Perform PubSub ops
	_ = store.PubSub().Publish(ctx, "metrics:chan", []byte("msg"))

	// Gather metrics
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("reg.Gather failed: %v", err)
	}

	names := make(map[string]bool)
	for _, mf := range mfs {
		names[mf.GetName()] = true
	}

	expected := []string{
		"goy_store_kv_operation_duration_seconds",
		"goy_store_sorted_set_operation_duration_seconds",
		"goy_store_blob_operation_duration_seconds",
		"goy_store_pubsub_operation_duration_seconds",
	}

	for _, exp := range expected {
		if !names[exp] {
			t.Errorf("expected metric %s to be registered and observed", exp)
		}
	}
}

func TestResilienceAndCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(2, 50*time.Millisecond)

	if err := cb.CanExecute(); err != nil {
		t.Fatalf("expected can execute on closed cb: %v", err)
	}

	cb.OnFailure()
	if err := cb.CanExecute(); err != nil {
		t.Fatalf("expected can execute after 1 failure: %v", err)
	}

	cb.OnFailure()
	// Should be open now
	if err := cb.CanExecute(); err == nil {
		t.Fatalf("expected circuit breaker to be OPEN")
	}

	time.Sleep(60 * time.Millisecond)
	// Should be half-open
	if err := cb.CanExecute(); err != nil {
		t.Fatalf("expected half-open cb to allow probe: %v", err)
	}

	cb.OnSuccess()
	// Should be closed
	if err := cb.CanExecute(); err != nil {
		t.Fatalf("expected closed cb after success: %v", err)
	}
}

func TestResilientExecutorRetry(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := RegisterMetrics(reg)

	cfg := ResilienceConfig{
		MaxRetries:                 2,
		BaseBackoffMS:              10,
		CircuitBreakerThreshold:    5,
		CircuitBreakerResetSeconds: 10,
		OperationTimeoutSeconds:    2,
	}

	exec := NewResilientExecutor(cfg, metrics, "test", "mem")

	attempt := 0
	ctx := context.Background()
	res, err := ExecuteWithResilience(exec, ctx, "retry_op", func(c context.Context) (int, error) {
		attempt++
		if attempt == 1 {
			return 0, errors.New("temporary error")
		}
		return 100, nil
	})

	if err != nil {
		t.Fatalf("expected success on retry: %v", err)
	}
	if res != 100 {
		t.Fatalf("expected result 100, got %d", res)
	}

	mfs, _ := reg.Gather()
	var retries float64
	for _, mf := range mfs {
		if mf.GetName() == "goy_store_retries_total" {
			for _, m := range mf.GetMetric() {
				retries += m.GetCounter().GetValue()
			}
		}
	}

	if retries != 1 {
		t.Fatalf("expected 1 retry recorded, got %v", retries)
	}
}

func TestHealthCheck(t *testing.T) {
	cfg := DefaultConfig()
	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("failed to initialize store: %v", err)
	}

	ctx := context.Background()
	health := store.HealthCheck(ctx)

	if health.State != HealthHealthy {
		t.Fatalf("expected healthy state, got %s", health.State)
	}

	if len(health.Contracts) != 5 {
		t.Fatalf("expected 5 contracts in health report, got %d", len(health.Contracts))
	}

	for _, name := range []string{"kv", "relational", "sorted_set", "pubsub", "blob"} {
		status, ok := health.Contracts[name]
		if !ok {
			t.Fatalf("expected contract %s in health report", name)
		}
		if status.State != HealthHealthy {
			t.Fatalf("expected %s to be healthy, got %s", name, status.State)
		}
	}
}
