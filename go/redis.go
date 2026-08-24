package goystore

import (
	"context"
	"errors"
	"fmt"
	"sync"
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

// Client returns the underlying redis.UniversalClient for connection sharing.
func (r *RedisKVStore) Client() redis.UniversalClient {
	return r.client
}

// IsHealthy executes a PING check on Redis with a timeout.
func (r *RedisKVStore) IsHealthy(ctx context.Context) (*HealthStatus, error) {
	start := time.Now()
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	err := r.client.Ping(pingCtx).Err()
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return &HealthStatus{
			Contract:  "kv",
			Backend:   "redis",
			State:     HealthUnhealthy,
			Message:   err.Error(),
			LatencyMS: latency,
		}, nil
	}

	return &HealthStatus{
		Contract:  "kv",
		Backend:   "redis",
		State:     HealthHealthy,
		LatencyMS: latency,
	}, nil
}

// RedisSortedSetStore implements the SortedSetStore contract using Redis.
type RedisSortedSetStore struct {
	client redis.UniversalClient
}

// NewRedisSortedSetStore creates a new RedisSortedSetStore from SortedSetConfig.
func NewRedisSortedSetStore(cfg SortedSetConfig) (*RedisSortedSetStore, error) {
	url := cfg.URL
	if url == "" {
		url = "redis://127.0.0.1:6379"
	}

	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opt)
	return &RedisSortedSetStore{client: client}, nil
}

// NewRedisSortedSetStoreWithClient creates a RedisSortedSetStore with a shared UniversalClient.
func NewRedisSortedSetStoreWithClient(client redis.UniversalClient) *RedisSortedSetStore {
	return &RedisSortedSetStore{client: client}
}

// Client returns the underlying redis.UniversalClient.
func (s *RedisSortedSetStore) Client() redis.UniversalClient {
	return s.client
}

// Add adds a member with a score to the sorted set.
func (s *RedisSortedSetStore) Add(ctx context.Context, set string, member string, score float64) error {
	return s.client.ZAdd(ctx, set, redis.Z{
		Score:  score,
		Member: member,
	}).Err()
}

// Remove removes a member from the sorted set.
func (s *RedisSortedSetStore) Remove(ctx context.Context, set string, member string) error {
	return s.client.ZRem(ctx, set, member).Err()
}

// RangeByScore retrieves members ordered by score within the [min, max] range.
func (s *RedisSortedSetStore) RangeByScore(ctx context.Context, set string, min, max float64, limit *int) ([]ScoredMember, error) {
	opt := &redis.ZRangeBy{
		Min: fmt.Sprintf("%f", min),
		Max: fmt.Sprintf("%f", max),
	}
	if limit != nil {
		opt.Offset = 0
		opt.Count = int64(*limit)
	}

	zs, err := s.client.ZRangeByScoreWithScores(ctx, set, opt).Result()
	if err != nil {
		return nil, err
	}

	members := make([]ScoredMember, len(zs))
	for i, z := range zs {
		memberStr, ok := z.Member.(string)
		if !ok {
			memberStr = fmt.Sprintf("%v", z.Member)
		}
		members[i] = ScoredMember{
			Member: memberStr,
			Score:  z.Score,
		}
	}
	return members, nil
}

// Count returns the number of elements in the sorted set.
func (s *RedisSortedSetStore) Count(ctx context.Context, set string) (int64, error) {
	return s.client.ZCard(ctx, set).Result()
}

// RemoveRange removes all members with scores in the range [min, max].
func (s *RedisSortedSetStore) RemoveRange(ctx context.Context, set string, min, max float64) (int64, error) {
	return s.client.ZRemRangeByScore(ctx, set, fmt.Sprintf("%f", min), fmt.Sprintf("%f", max)).Result()
}

// Score returns the score of the specified member in the sorted set, or nil if not found.
func (s *RedisSortedSetStore) Score(ctx context.Context, set string, member string) (*float64, error) {
	score, err := s.client.ZScore(ctx, set, member).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &score, nil
}

// IsHealthy executes a PING check on Redis with a timeout.
func (s *RedisSortedSetStore) IsHealthy(ctx context.Context) (*HealthStatus, error) {
	start := time.Now()
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	err := s.client.Ping(pingCtx).Err()
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return &HealthStatus{
			Contract:  "sorted_set",
			Backend:   "redis",
			State:     HealthUnhealthy,
			Message:   err.Error(),
			LatencyMS: latency,
		}, nil
	}

	return &HealthStatus{
		Contract:  "sorted_set",
		Backend:   "redis",
		State:     HealthHealthy,
		LatencyMS: latency,
	}, nil
}

// RedisPubSubStore implements the PubSubStore contract using Redis.
type RedisPubSubStore struct {
	client redis.UniversalClient
	mu     sync.Mutex
	subs   map[string][]*redis.PubSub
}

// NewRedisPubSubStore creates a new RedisPubSubStore from PubSubConfig.
func NewRedisPubSubStore(cfg PubSubConfig) (*RedisPubSubStore, error) {
	url := cfg.URL
	if url == "" {
		url = "redis://127.0.0.1:6379"
	}

	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opt)
	return &RedisPubSubStore{
		client: client,
		subs:   make(map[string][]*redis.PubSub),
	}, nil
}

// NewRedisPubSubStoreWithClient creates a RedisPubSubStore with a shared UniversalClient.
func NewRedisPubSubStoreWithClient(client redis.UniversalClient) *RedisPubSubStore {
	return &RedisPubSubStore{
		client: client,
		subs:   make(map[string][]*redis.PubSub),
	}
}

// Publish sends a message to a channel.
func (p *RedisPubSubStore) Publish(ctx context.Context, channel string, message []byte) error {
	return p.client.Publish(ctx, channel, message).Err()
}

// Subscribe subscribes to channels and returns a buffered receive-only channel.
func (p *RedisPubSubStore) Subscribe(ctx context.Context, channels []string) (<-chan Message, error) {
	if len(channels) == 0 {
		return nil, fmt.Errorf("no channels provided for subscription")
	}

	pubsub := p.client.Subscribe(ctx, channels...)
	// Confirm subscription
	_, err := pubsub.Receive(ctx)
	if err != nil {
		_ = pubsub.Close()
		return nil, fmt.Errorf("failed to initialize subscription: %w", err)
	}

	p.mu.Lock()
	for _, ch := range channels {
		p.subs[ch] = append(p.subs[ch], pubsub)
	}
	p.mu.Unlock()

	out := make(chan Message, 1000)
	ch := pubsub.Channel()

	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				_ = pubsub.Close()
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				out <- Message{
					Channel:   msg.Channel,
					Payload:   []byte(msg.Payload),
					Timestamp: time.Now(),
				}
			}
		}
	}()

	return out, nil
}

// Unsubscribe cancels subscriptions on the given channels.
func (p *RedisPubSubStore) Unsubscribe(ctx context.Context, channels []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, ch := range channels {
		if subsList, ok := p.subs[ch]; ok {
			for _, sub := range subsList {
				_ = sub.Unsubscribe(ctx, ch)
				_ = sub.Close()
			}
			delete(p.subs, ch)
		}
	}
	return nil
}

// IsHealthy executes a PING check on Redis with a timeout.
func (p *RedisPubSubStore) IsHealthy(ctx context.Context) (*HealthStatus, error) {
	start := time.Now()
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	err := p.client.Ping(pingCtx).Err()
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return &HealthStatus{
			Contract:  "pubsub",
			Backend:   "redis",
			State:     HealthUnhealthy,
			Message:   err.Error(),
			LatencyMS: latency,
		}, nil
	}

	return &HealthStatus{
		Contract:  "pubsub",
		Backend:   "redis",
		State:     HealthHealthy,
		LatencyMS: latency,
	}, nil
}
