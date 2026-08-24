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

	memStore := NewMemoryStore().(*MemoryStore)

	return &defaultStore{
		kv:         kv,
		relational: relational,
		sortedSet:  memStore.sortedSet,
		pubsub:     memStore.pubsub,
		blob:       memStore.blob,
	}, nil
}
