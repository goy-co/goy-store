package goystore

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// CircuitState represents the current state of a circuit breaker.
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// CircuitBreaker protects calls against cascading failures.
type CircuitBreaker struct {
	mu                  sync.RWMutex
	state               CircuitState
	consecutiveFailures atomic.Int32
	failureThreshold    int32
	resetTimeout        time.Duration
	lastFailureTime     time.Time
}

// NewCircuitBreaker creates a new CircuitBreaker.
func NewCircuitBreaker(threshold int, resetTimeout time.Duration) *CircuitBreaker {
	if threshold <= 0 {
		threshold = 5
	}
	if resetTimeout <= 0 {
		resetTimeout = 30 * time.Second
	}
	return &CircuitBreaker{
		state:            CircuitClosed,
		failureThreshold: int32(threshold),
		resetTimeout:     resetTimeout,
	}
}

// CanExecute returns an error if the circuit is open.
func (cb *CircuitBreaker) CanExecute() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return nil
	case CircuitOpen:
		if time.Since(cb.lastFailureTime) >= cb.resetTimeout {
			cb.state = CircuitHalfOpen
			return nil
		}
		return errors.New("circuit breaker is OPEN")
	case CircuitHalfOpen:
		return nil
	default:
		return nil
	}
}

// OnSuccess records a successful execution, resetting the circuit to Closed.
func (cb *CircuitBreaker) OnSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFailures.Store(0)
	cb.state = CircuitClosed
}

// OnFailure records a failure, potentially opening the circuit.
func (cb *CircuitBreaker) OnFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailureTime = time.Now()
	failures := cb.consecutiveFailures.Add(1)
	if failures >= cb.failureThreshold {
		cb.state = CircuitOpen
	}
}

// StateCode returns numeric code for metrics (0=closed, 1=open, 2=half-open).
func (cb *CircuitBreaker) StateCode() float64 {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitClosed:
		return 0.0
	case CircuitOpen:
		return 1.0
	case CircuitHalfOpen:
		return 2.0
	default:
		return 0.0
	}
}

// ResilientExecutor wraps operations with timeout, retry with exponential backoff & jitter, and circuit breaker.
type ResilientExecutor struct {
	cb       *CircuitBreaker
	cfg      ResilienceConfig
	metrics  *Metrics
	contract string
	backend  string
}

// NewResilientExecutor creates a new ResilientExecutor.
func NewResilientExecutor(cfg ResilienceConfig, metrics *Metrics, contract, backend string) *ResilientExecutor {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.BaseBackoffMS <= 0 {
		cfg.BaseBackoffMS = 100
	}
	if cfg.CircuitBreakerThreshold <= 0 {
		cfg.CircuitBreakerThreshold = 5
	}
	if cfg.CircuitBreakerResetSeconds <= 0 {
		cfg.CircuitBreakerResetSeconds = 30
	}
	if cfg.OperationTimeoutSeconds <= 0 {
		cfg.OperationTimeoutSeconds = 5
	}

	cb := NewCircuitBreaker(cfg.CircuitBreakerThreshold, time.Duration(cfg.CircuitBreakerResetSeconds)*time.Second)
	return &ResilientExecutor{
		cb:       cb,
		cfg:      cfg,
		metrics:  metrics,
		contract: contract,
		backend:  backend,
	}
}

// Execute runs the given operation with resilience protection.
func ExecuteWithResilience[T any](exec *ResilientExecutor, parentCtx context.Context, opName string, fn func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	if err := exec.cb.CanExecute(); err != nil {
		return zero, err
	}

	timeout := time.Duration(exec.cfg.OperationTimeoutSeconds) * time.Second
	maxRetries := exec.cfg.MaxRetries
	baseBackoff := time.Duration(exec.cfg.BaseBackoffMS) * time.Millisecond

	var lastErr error
	for attempt := 1; attempt <= maxRetries+1; attempt++ {
		opCtx, cancel := context.WithTimeout(parentCtx, timeout)
		val, err := fn(opCtx)
		cancel()

		if err == nil {
			exec.cb.OnSuccess()
			if exec.metrics != nil {
				exec.metrics.CircuitBreakerState.WithLabelValues(exec.contract, exec.backend).Set(exec.cb.StateCode())
			}
			return val, nil
		}

		lastErr = err
		exec.cb.OnFailure()
		if exec.metrics != nil {
			exec.metrics.CircuitBreakerState.WithLabelValues(exec.contract, exec.backend).Set(exec.cb.StateCode())
		}

		if attempt <= maxRetries {
			if exec.metrics != nil {
				exec.metrics.RetriesTotal.WithLabelValues(exec.contract, opName, exec.backend).Inc()
			}

			// Exponential backoff with ±25% jitter
			shift := uint(attempt - 1)
			factor := float64(int(1) << shift)
			jitter := 0.75 + rand.Float64()*0.5
			sleepDur := time.Duration(float64(baseBackoff) * factor * jitter)

			select {
			case <-parentCtx.Done():
				return zero, parentCtx.Err()
			case <-time.After(sleepDur):
			}
		}
	}

	return zero, fmt.Errorf("operation %s failed after %d retries: %w", opName, maxRetries, lastErr)
}

// --- Resilient KV Store Wrapper ---

type resilientKVStore struct {
	inner    KVStore
	executor *ResilientExecutor
}

func WrapKVWithResilience(inner KVStore, cfg ResilienceConfig, metrics *Metrics, backend string) KVStore {
	return &resilientKVStore{
		inner:    inner,
		executor: NewResilientExecutor(cfg, metrics, "kv", backend),
	}
}

func (r *resilientKVStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	type getRes struct {
		val []byte
		ok  bool
	}
	res, err := ExecuteWithResilience(r.executor, ctx, "get", func(c context.Context) (getRes, error) {
		val, ok, err := r.inner.Get(c, key)
		return getRes{val: val, ok: ok}, err
	})
	return res.val, res.ok, err
}

func (r *resilientKVStore) Set(ctx context.Context, key string, value []byte, ttl *time.Duration) error {
	_, err := ExecuteWithResilience(r.executor, ctx, "set", func(c context.Context) (struct{}, error) {
		return struct{}{}, r.inner.Set(c, key, value, ttl)
	})
	return err
}

func (r *resilientKVStore) Delete(ctx context.Context, key string) error {
	_, err := ExecuteWithResilience(r.executor, ctx, "delete", func(c context.Context) (struct{}, error) {
		return struct{}{}, r.inner.Delete(c, key)
	})
	return err
}

func (r *resilientKVStore) Exists(ctx context.Context, key string) (bool, error) {
	return ExecuteWithResilience(r.executor, ctx, "exists", func(c context.Context) (bool, error) {
		return r.inner.Exists(c, key)
	})
}

func (r *resilientKVStore) SetIfNotExists(ctx context.Context, key string, value []byte, ttl *time.Duration) (bool, error) {
	return ExecuteWithResilience(r.executor, ctx, "set_if_not_exists", func(c context.Context) (bool, error) {
		return r.inner.SetIfNotExists(c, key, value, ttl)
	})
}

// --- Resilient Relational Store Wrapper ---

type resilientRelationalStore struct {
	inner    RelationalStore
	executor *ResilientExecutor
}

func WrapRelationalWithResilience(inner RelationalStore, cfg ResilienceConfig, metrics *Metrics, backend string) RelationalStore {
	return &resilientRelationalStore{
		inner:    inner,
		executor: NewResilientExecutor(cfg, metrics, "relational", backend),
	}
}

func (r *resilientRelationalStore) Query(ctx context.Context, sql string, params []any) (Rows, error) {
	return ExecuteWithResilience(r.executor, ctx, "query", func(c context.Context) (Rows, error) {
		return r.inner.Query(c, sql, params)
	})
}

func (r *resilientRelationalStore) Execute(ctx context.Context, sql string, params []any) (int64, error) {
	return ExecuteWithResilience(r.executor, ctx, "execute", func(c context.Context) (int64, error) {
		return r.inner.Execute(c, sql, params)
	})
}

func (r *resilientRelationalStore) Transaction(ctx context.Context, fn func(Tx) error) error {
	_, err := ExecuteWithResilience(r.executor, ctx, "transaction", func(c context.Context) (struct{}, error) {
		return struct{}{}, r.inner.Transaction(c, fn)
	})
	return err
}

func (r *resilientRelationalStore) Migrate(ctx context.Context, migrations []Migration) error {
	_, err := ExecuteWithResilience(r.executor, ctx, "migrate", func(c context.Context) (struct{}, error) {
		return struct{}{}, r.inner.Migrate(c, migrations)
	})
	return err
}

// --- Resilient Sorted Set Store Wrapper ---

type resilientSortedSetStore struct {
	inner    SortedSetStore
	executor *ResilientExecutor
}

func WrapSortedSetWithResilience(inner SortedSetStore, cfg ResilienceConfig, metrics *Metrics, backend string) SortedSetStore {
	return &resilientSortedSetStore{
		inner:    inner,
		executor: NewResilientExecutor(cfg, metrics, "sorted_set", backend),
	}
}

func (r *resilientSortedSetStore) Add(ctx context.Context, set string, member string, score float64) error {
	_, err := ExecuteWithResilience(r.executor, ctx, "add", func(c context.Context) (struct{}, error) {
		return struct{}{}, r.inner.Add(c, set, member, score)
	})
	return err
}

func (r *resilientSortedSetStore) Remove(ctx context.Context, set string, member string) error {
	_, err := ExecuteWithResilience(r.executor, ctx, "remove", func(c context.Context) (struct{}, error) {
		return struct{}{}, r.inner.Remove(c, set, member)
	})
	return err
}

func (r *resilientSortedSetStore) RangeByScore(ctx context.Context, set string, min, max float64, limit *int) ([]ScoredMember, error) {
	return ExecuteWithResilience(r.executor, ctx, "range_by_score", func(c context.Context) ([]ScoredMember, error) {
		return r.inner.RangeByScore(c, set, min, max, limit)
	})
}

func (r *resilientSortedSetStore) Count(ctx context.Context, set string) (int64, error) {
	return ExecuteWithResilience(r.executor, ctx, "count", func(c context.Context) (int64, error) {
		return r.inner.Count(c, set)
	})
}

func (r *resilientSortedSetStore) RemoveRange(ctx context.Context, set string, min, max float64) (int64, error) {
	return ExecuteWithResilience(r.executor, ctx, "remove_range", func(c context.Context) (int64, error) {
		return r.inner.RemoveRange(c, set, min, max)
	})
}

func (r *resilientSortedSetStore) Score(ctx context.Context, set string, member string) (*float64, error) {
	return ExecuteWithResilience(r.executor, ctx, "score", func(c context.Context) (*float64, error) {
		return r.inner.Score(c, set, member)
	})
}

// --- Resilient Blob Store Wrapper ---

type resilientBlobStore struct {
	inner    BlobStore
	executor *ResilientExecutor
}

func WrapBlobWithResilience(inner BlobStore, cfg ResilienceConfig, metrics *Metrics, backend string) BlobStore {
	return &resilientBlobStore{
		inner:    inner,
		executor: NewResilientExecutor(cfg, metrics, "blob", backend),
	}
}

func (r *resilientBlobStore) Put(ctx context.Context, key string, data []byte, metadata *Metadata) error {
	_, err := ExecuteWithResilience(r.executor, ctx, "put", func(c context.Context) (struct{}, error) {
		return struct{}{}, r.inner.Put(c, key, data, metadata)
	})
	return err
}

func (r *resilientBlobStore) Get(ctx context.Context, key string) ([]byte, *Metadata, bool, error) {
	type getRes struct {
		data []byte
		meta *Metadata
		ok   bool
	}
	res, err := ExecuteWithResilience(r.executor, ctx, "get", func(c context.Context) (getRes, error) {
		data, meta, ok, err := r.inner.Get(c, key)
		return getRes{data: data, meta: meta, ok: ok}, err
	})
	return res.data, res.meta, res.ok, err
}

func (r *resilientBlobStore) Delete(ctx context.Context, key string) error {
	_, err := ExecuteWithResilience(r.executor, ctx, "delete", func(c context.Context) (struct{}, error) {
		return struct{}{}, r.inner.Delete(c, key)
	})
	return err
}

func (r *resilientBlobStore) List(ctx context.Context, prefix *string) ([]string, error) {
	return ExecuteWithResilience(r.executor, ctx, "list", func(c context.Context) ([]string, error) {
		return r.inner.List(c, prefix)
	})
}

func (r *resilientBlobStore) PresignURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return ExecuteWithResilience(r.executor, ctx, "presign_url", func(c context.Context) (string, error) {
		return r.inner.PresignURL(c, key, expiry)
	})
}

// --- Resilient PubSub Store Wrapper ---

type resilientPubSubStore struct {
	inner    PubSubStore
	executor *ResilientExecutor
}

func WrapPubSubWithResilience(inner PubSubStore, cfg ResilienceConfig, metrics *Metrics, backend string) PubSubStore {
	return &resilientPubSubStore{
		inner:    inner,
		executor: NewResilientExecutor(cfg, metrics, "pubsub", backend),
	}
}

func (r *resilientPubSubStore) Publish(ctx context.Context, channel string, message []byte) error {
	_, err := ExecuteWithResilience(r.executor, ctx, "publish", func(c context.Context) (struct{}, error) {
		return struct{}{}, r.inner.Publish(c, channel, message)
	})
	return err
}

func (r *resilientPubSubStore) Subscribe(ctx context.Context, channels []string) (<-chan Message, error) {
	return r.inner.Subscribe(ctx, channels)
}

func (r *resilientPubSubStore) Unsubscribe(ctx context.Context, channels []string) error {
	return r.inner.Unsubscribe(ctx, channels)
}
