package goystore

import (
	"context"
	"fmt"
)

type defaultStore struct {
	kv         KVStore
	relational RelationalStore
	sortedSet  SortedSetStore
	pubsub     PubSubStore
	blob       BlobStore
	metrics    *Metrics
}

func (s *defaultStore) KV() KVStore                 { return s.kv }
func (s *defaultStore) Relational() RelationalStore { return s.relational }
func (s *defaultStore) SortedSet() SortedSetStore   { return s.sortedSet }
func (s *defaultStore) PubSub() PubSubStore         { return s.pubsub }
func (s *defaultStore) Blob() BlobStore             { return s.blob }
func (s *defaultStore) Metrics() *Metrics           { return s.metrics }

func (s *defaultStore) HealthCheck(ctx context.Context) ConsolidatedHealth {
	type checkResult struct {
		contract string
		status   *HealthStatus
	}

	results := make(chan checkResult, 5)

	go func() {
		status, err := s.kv.IsHealthy(ctx)
		if err != nil && status == nil {
			status = &HealthStatus{Contract: "kv", Backend: "unknown", State: HealthUnhealthy, Message: err.Error()}
		}
		results <- checkResult{contract: "kv", status: status}
	}()

	go func() {
		status, err := s.relational.IsHealthy(ctx)
		if err != nil && status == nil {
			status = &HealthStatus{Contract: "relational", Backend: "unknown", State: HealthUnhealthy, Message: err.Error()}
		}
		results <- checkResult{contract: "relational", status: status}
	}()

	go func() {
		status, err := s.sortedSet.IsHealthy(ctx)
		if err != nil && status == nil {
			status = &HealthStatus{Contract: "sorted_set", Backend: "unknown", State: HealthUnhealthy, Message: err.Error()}
		}
		results <- checkResult{contract: "sorted_set", status: status}
	}()

	go func() {
		status, err := s.pubsub.IsHealthy(ctx)
		if err != nil && status == nil {
			status = &HealthStatus{Contract: "pubsub", Backend: "unknown", State: HealthUnhealthy, Message: err.Error()}
		}
		results <- checkResult{contract: "pubsub", status: status}
	}()

	go func() {
		status, err := s.blob.IsHealthy(ctx)
		if err != nil && status == nil {
			status = &HealthStatus{Contract: "blob", Backend: "unknown", State: HealthUnhealthy, Message: err.Error()}
		}
		results <- checkResult{contract: "blob", status: status}
	}()

	contracts := make(map[string]HealthStatus, 5)
	consolidatedState := HealthHealthy

	for i := 0; i < 5; i++ {
		res := <-results
		if res.status != nil {
			contracts[res.contract] = *res.status
			if res.status.State == HealthUnhealthy {
				consolidatedState = HealthUnhealthy
			} else if res.status.State == HealthDegraded && consolidatedState != HealthUnhealthy {
				consolidatedState = HealthDegraded
			}
		}
	}

	return ConsolidatedHealth{
		State:     consolidatedState,
		Contracts: contracts,
	}
}

// NewStore creates a new GoyStore instance from the provided configuration with default metrics.
func NewStore(cfg *Config) (GoyStore, error) {
	return NewStoreWithMetrics(cfg, DefaultMetrics())
}

// NewStoreWithMetrics creates a new GoyStore instance from the provided configuration with custom metrics.
func NewStoreWithMetrics(cfg *Config, metrics *Metrics) (GoyStore, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if metrics == nil {
		metrics = DefaultMetrics()
	}

	var kv KVStore
	switch cfg.KV.Backend {
	case "redis":
		rkv, err := NewRedisKVStore(cfg.KV)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize redis kv store: %w", err)
		}
		kv = rkv
	case "memory", "":
		kv = &memoryKV{data: make(map[string][]byte)}
	default:
		return nil, fmt.Errorf("unsupported kv backend: %s", cfg.KV.Backend)
	}

	var relational RelationalStore
	switch cfg.Relational.Backend {
	case "postgres":
		pg, err := NewPostgresRelationalStore(context.Background(), cfg.Relational)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize postgres relational store: %w", err)
		}
		relational = pg
	case "memory", "":
		relational = &memoryRelational{}
	default:
		return nil, fmt.Errorf("unsupported relational backend: %s", cfg.Relational.Backend)
	}

	var sortedSet SortedSetStore
	switch cfg.SortedSet.Backend {
	case "redis":
		if rkv, ok := kv.(*RedisKVStore); ok && (cfg.SortedSet.URL == "" || cfg.SortedSet.URL == cfg.KV.URL) {
			sortedSet = NewRedisSortedSetStoreWithClient(rkv.Client())
		} else {
			rss, err := NewRedisSortedSetStore(cfg.SortedSet)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize redis sorted set store: %w", err)
			}
			sortedSet = rss
		}
	case "memory", "":
		sortedSet = &memorySortedSet{sets: make(map[string]map[string]float64)}
	default:
		return nil, fmt.Errorf("unsupported sorted_set backend: %s", cfg.SortedSet.Backend)
	}

	var blob BlobStore
	switch cfg.Blob.Backend {
	case "local", "filesystem":
		path := cfg.Blob.Path
		if path == "" {
			path = "./data/blobs"
		}
		blob = NewLocalBlobStore(path)
	case "s3", "minio", "r2":
		s3Store, err := NewS3BlobStore(context.Background(), cfg.Blob)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize s3 blob store: %w", err)
		}
		blob = s3Store
	case "memory", "":
		blob = &memoryBlob{blobs: make(map[string]blobData)}
	default:
		return nil, fmt.Errorf("unsupported blob backend: %s", cfg.Blob.Backend)
	}

	var pubsub PubSubStore
	switch cfg.PubSub.Backend {
	case "redis":
		if rkv, ok := kv.(*RedisKVStore); ok && (cfg.PubSub.URL == "" || cfg.PubSub.URL == cfg.KV.URL) {
			pubsub = NewRedisPubSubStoreWithClient(rkv.Client())
		} else {
			rps, err := NewRedisPubSubStore(cfg.PubSub)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize redis pubsub store: %w", err)
			}
			pubsub = rps
		}
	case "memory", "":
		pubsub = &memoryPubSub{subscribers: make(map[string]map[chan Message]struct{})}
	default:
		return nil, fmt.Errorf("unsupported pubsub backend: %s", cfg.PubSub.Backend)
	}

	// 1. Wrap with metrics
	kvMetric := WrapKVWithMetrics(kv, metrics, cfg.KV.Backend)
	relationalMetric := WrapRelationalWithMetrics(relational, metrics, cfg.Relational.Backend)
	sortedSetMetric := WrapSortedSetWithMetrics(sortedSet, metrics, cfg.SortedSet.Backend)
	pubsubMetric := WrapPubSubWithMetrics(pubsub, metrics, cfg.PubSub.Backend)
	blobMetric := WrapBlobWithMetrics(blob, metrics, cfg.Blob.Backend)

	// 2. Wrap with resilience
	return &defaultStore{
		kv:         WrapKVWithResilience(kvMetric, cfg.Resilience, metrics, cfg.KV.Backend),
		relational: WrapRelationalWithResilience(relationalMetric, cfg.Resilience, metrics, cfg.Relational.Backend),
		sortedSet:  WrapSortedSetWithResilience(sortedSetMetric, cfg.Resilience, metrics, cfg.SortedSet.Backend),
		pubsub:     WrapPubSubWithResilience(pubsubMetric, cfg.Resilience, metrics, cfg.PubSub.Backend),
		blob:       WrapBlobWithResilience(blobMetric, cfg.Resilience, metrics, cfg.Blob.Backend),
		metrics:    metrics,
	}, nil
}
