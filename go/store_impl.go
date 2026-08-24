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
}

func (s *defaultStore) KV() KVStore          { return s.kv }
func (s *defaultStore) Relational() RelationalStore { return s.relational }
func (s *defaultStore) SortedSet() SortedSetStore   { return s.sortedSet }
func (s *defaultStore) PubSub() PubSubStore         { return s.pubsub }
func (s *defaultStore) Blob() BlobStore             { return s.blob }

// NewStore creates a new GoyStore instance from the provided configuration.
func NewStore(cfg *Config) (GoyStore, error) {
	if cfg == nil {
		cfg = DefaultConfig()
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

	return &defaultStore{
		kv:         kv,
		relational: relational,
		sortedSet:  sortedSet,
		pubsub:     pubsub,
		blob:       blob,
	}, nil
}
