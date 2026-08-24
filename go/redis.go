package goystore

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisKVStore implements the KVStore contract using Redis.
type RedisKVStore struct {
	client redis.UniversalClient
}

// NewRedisKVStore creates a new RedisKVStore from KVConfig.
func NewRedisKVStore(cfg KVConfig) (*RedisKVStore, error) {
	url := cfg.URL
	if url == "" {
		url = "redis://127.0.0.1:6379"
	}

	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}

	if cfg.PoolSize > 0 {
		opt.PoolSize = cfg.PoolSize
	}
	if cfg.TimeoutMS > 0 {
		timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
		opt.ReadTimeout = timeout
		opt.WriteTimeout = timeout
		opt.DialTimeout = timeout
	}

	client := redis.NewClient(opt)
	return &RedisKVStore{client: client}, nil
}

// NewRedisKVStoreWithClient creates a RedisKVStore with a preconfigured UniversalClient.
func NewRedisKVStoreWithClient(client redis.UniversalClient) *RedisKVStore {
	return &RedisKVStore{client: client}
}

// Get retrieves the value for key. Returns (nil, false, nil) if key does not exist.
func (r *RedisKVStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	val, err := r.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return val, true, nil
}

// Set sets the key to value with an optional TTL.
func (r *RedisKVStore) Set(ctx context.Context, key string, value []byte, ttl *time.Duration) error {
	var expiration time.Duration
	if ttl != nil {
		expiration = *ttl
	}
	return r.client.Set(ctx, key, value, expiration).Err()
}

// Delete removes key from the store.
func (r *RedisKVStore) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

// Exists checks if key exists in the store.
func (r *RedisKVStore) Exists(ctx context.Context, key string) (bool, error) {
	count, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// SetIfNotExists sets the key to value if it does not already exist, returning true if set.
func (r *RedisKVStore) SetIfNotExists(ctx context.Context, key string, value []byte, ttl *time.Duration) (bool, error) {
	var expiration time.Duration
	if ttl != nil {
		expiration = *ttl
	}
	return r.client.SetNX(ctx, key, value, expiration).Result()
}
