package goystore

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds the Prometheus metrics for Goy Store.
type Metrics struct {
	KVOperationDuration        *prometheus.HistogramVec
	RelationalQueryDuration    *prometheus.HistogramVec
	SortedSetOperationDuration *prometheus.HistogramVec
	BlobOperationDuration      *prometheus.HistogramVec
	PubSubOperationDuration    *prometheus.HistogramVec
	ErrorsTotal                *prometheus.CounterVec
	PoolActiveConnections      *prometheus.GaugeVec
	PoolIdleConnections        *prometheus.GaugeVec
}

var (
	defaultMetrics     *Metrics
	defaultMetricsOnce sync.Once
)

// RegisterMetrics initializes and registers Goy Store metrics with the given prometheus.Registerer.
// If reg is nil, prometheus.DefaultRegisterer is used.
func RegisterMetrics(reg prometheus.Registerer) *Metrics {
	factory := promauto.With(reg)

	return &Metrics{
		KVOperationDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "goy_store_kv_operation_duration_seconds",
				Help:    "Duration of KV store operations in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation", "backend"},
		),
		RelationalQueryDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "goy_store_relational_query_duration_seconds",
				Help:    "Duration of relational query operations in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation", "backend"},
		),
		SortedSetOperationDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "goy_store_sorted_set_operation_duration_seconds",
				Help:    "Duration of sorted set operations in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation", "backend"},
		),
		BlobOperationDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "goy_store_blob_operation_duration_seconds",
				Help:    "Duration of blob operations in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation", "backend"},
		),
		PubSubOperationDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "goy_store_pubsub_operation_duration_seconds",
				Help:    "Duration of pubsub operations in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation", "backend"},
		),
		ErrorsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "goy_store_errors_total",
				Help: "Total number of store errors",
			},
			[]string{"contract", "operation", "backend", "error_type"},
		),
		PoolActiveConnections: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "goy_store_pool_active_connections",
				Help: "Number of active pool connections",
			},
			[]string{"backend"},
		),
		PoolIdleConnections: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "goy_store_pool_idle_connections",
				Help: "Number of idle pool connections",
			},
			[]string{"backend"},
		),
	}
}

// DefaultMetrics returns the singleton metrics instance registered on the default Prometheus registry.
func DefaultMetrics() *Metrics {
	defaultMetricsOnce.Do(func() {
		defaultMetrics = RegisterMetrics(prometheus.DefaultRegisterer)
	})
	return defaultMetrics
}

// --- Instrumented KV Store ---

type instrumentedKVStore struct {
	inner   KVStore
	metrics *Metrics
	backend string
}

func WrapKVWithMetrics(inner KVStore, metrics *Metrics, backend string) KVStore {
	if metrics == nil {
		metrics = DefaultMetrics()
	}
	return &instrumentedKVStore{inner: inner, metrics: metrics, backend: backend}
}

func (i *instrumentedKVStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	start := time.Now()
	val, ok, err := i.inner.Get(ctx, key)
	i.metrics.KVOperationDuration.WithLabelValues("get", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("kv", "get", i.backend, "error").Inc()
	}
	return val, ok, err
}

func (i *instrumentedKVStore) Set(ctx context.Context, key string, value []byte, ttl *time.Duration) error {
	start := time.Now()
	err := i.inner.Set(ctx, key, value, ttl)
	i.metrics.KVOperationDuration.WithLabelValues("set", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("kv", "set", i.backend, "error").Inc()
	}
	return err
}

func (i *instrumentedKVStore) Delete(ctx context.Context, key string) error {
	start := time.Now()
	err := i.inner.Delete(ctx, key)
	i.metrics.KVOperationDuration.WithLabelValues("delete", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("kv", "delete", i.backend, "error").Inc()
	}
	return err
}

func (i *instrumentedKVStore) Exists(ctx context.Context, key string) (bool, error) {
	start := time.Now()
	exists, err := i.inner.Exists(ctx, key)
	i.metrics.KVOperationDuration.WithLabelValues("exists", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("kv", "exists", i.backend, "error").Inc()
	}
	return exists, err
}

func (i *instrumentedKVStore) SetIfNotExists(ctx context.Context, key string, value []byte, ttl *time.Duration) (bool, error) {
	start := time.Now()
	set, err := i.inner.SetIfNotExists(ctx, key, value, ttl)
	i.metrics.KVOperationDuration.WithLabelValues("set_if_not_exists", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("kv", "set_if_not_exists", i.backend, "error").Inc()
	}
	return set, err
}

// --- Instrumented Relational Store ---

type instrumentedRelationalStore struct {
	inner   RelationalStore
	metrics *Metrics
	backend string
}

func WrapRelationalWithMetrics(inner RelationalStore, metrics *Metrics, backend string) RelationalStore {
	if metrics == nil {
		metrics = DefaultMetrics()
	}
	return &instrumentedRelationalStore{inner: inner, metrics: metrics, backend: backend}
}

func (i *instrumentedRelationalStore) Query(ctx context.Context, sql string, params []any) (Rows, error) {
	start := time.Now()
	rows, err := i.inner.Query(ctx, sql, params)
	i.metrics.RelationalQueryDuration.WithLabelValues("query", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("relational", "query", i.backend, "error").Inc()
	}
	return rows, err
}

func (i *instrumentedRelationalStore) Execute(ctx context.Context, sql string, params []any) (int64, error) {
	start := time.Now()
	affected, err := i.inner.Execute(ctx, sql, params)
	i.metrics.RelationalQueryDuration.WithLabelValues("execute", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("relational", "execute", i.backend, "error").Inc()
	}
	return affected, err
}

func (i *instrumentedRelationalStore) Transaction(ctx context.Context, fn func(Tx) error) error {
	start := time.Now()
	err := i.inner.Transaction(ctx, fn)
	i.metrics.RelationalQueryDuration.WithLabelValues("transaction", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("relational", "transaction", i.backend, "error").Inc()
	}
	return err
}

func (i *instrumentedRelationalStore) Migrate(ctx context.Context, migrations []Migration) error {
	start := time.Now()
	err := i.inner.Migrate(ctx, migrations)
	i.metrics.RelationalQueryDuration.WithLabelValues("migrate", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("relational", "migrate", i.backend, "error").Inc()
	}
	return err
}

// --- Instrumented Sorted Set Store ---

type instrumentedSortedSetStore struct {
	inner   SortedSetStore
	metrics *Metrics
	backend string
}

func WrapSortedSetWithMetrics(inner SortedSetStore, metrics *Metrics, backend string) SortedSetStore {
	if metrics == nil {
		metrics = DefaultMetrics()
	}
	return &instrumentedSortedSetStore{inner: inner, metrics: metrics, backend: backend}
}

func (i *instrumentedSortedSetStore) Add(ctx context.Context, set string, member string, score float64) error {
	start := time.Now()
	err := i.inner.Add(ctx, set, member, score)
	i.metrics.SortedSetOperationDuration.WithLabelValues("add", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("sorted_set", "add", i.backend, "error").Inc()
	}
	return err
}

func (i *instrumentedSortedSetStore) Remove(ctx context.Context, set string, member string) error {
	start := time.Now()
	err := i.inner.Remove(ctx, set, member)
	i.metrics.SortedSetOperationDuration.WithLabelValues("remove", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("sorted_set", "remove", i.backend, "error").Inc()
	}
	return err
}

func (i *instrumentedSortedSetStore) RangeByScore(ctx context.Context, set string, min, max float64, limit *int) ([]ScoredMember, error) {
	start := time.Now()
	res, err := i.inner.RangeByScore(ctx, set, min, max, limit)
	i.metrics.SortedSetOperationDuration.WithLabelValues("range_by_score", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("sorted_set", "range_by_score", i.backend, "error").Inc()
	}
	return res, err
}

func (i *instrumentedSortedSetStore) Count(ctx context.Context, set string) (int64, error) {
	start := time.Now()
	count, err := i.inner.Count(ctx, set)
	i.metrics.SortedSetOperationDuration.WithLabelValues("count", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("sorted_set", "count", i.backend, "error").Inc()
	}
	return count, err
}

func (i *instrumentedSortedSetStore) RemoveRange(ctx context.Context, set string, min, max float64) (int64, error) {
	start := time.Now()
	count, err := i.inner.RemoveRange(ctx, set, min, max)
	i.metrics.SortedSetOperationDuration.WithLabelValues("remove_range", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("sorted_set", "remove_range", i.backend, "error").Inc()
	}
	return count, err
}

func (i *instrumentedSortedSetStore) Score(ctx context.Context, set string, member string) (*float64, error) {
	start := time.Now()
	score, err := i.inner.Score(ctx, set, member)
	i.metrics.SortedSetOperationDuration.WithLabelValues("score", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("sorted_set", "score", i.backend, "error").Inc()
	}
	return score, err
}

// --- Instrumented Blob Store ---

type instrumentedBlobStore struct {
	inner   BlobStore
	metrics *Metrics
	backend string
}

func WrapBlobWithMetrics(inner BlobStore, metrics *Metrics, backend string) BlobStore {
	if metrics == nil {
		metrics = DefaultMetrics()
	}
	return &instrumentedBlobStore{inner: inner, metrics: metrics, backend: backend}
}

func (i *instrumentedBlobStore) Put(ctx context.Context, key string, data []byte, metadata *Metadata) error {
	start := time.Now()
	err := i.inner.Put(ctx, key, data, metadata)
	i.metrics.BlobOperationDuration.WithLabelValues("put", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("blob", "put", i.backend, "error").Inc()
	}
	return err
}

func (i *instrumentedBlobStore) Get(ctx context.Context, key string) ([]byte, *Metadata, bool, error) {
	start := time.Now()
	data, meta, ok, err := i.inner.Get(ctx, key)
	i.metrics.BlobOperationDuration.WithLabelValues("get", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("blob", "get", i.backend, "error").Inc()
	}
	return data, meta, ok, err
}

func (i *instrumentedBlobStore) Delete(ctx context.Context, key string) error {
	start := time.Now()
	err := i.inner.Delete(ctx, key)
	i.metrics.BlobOperationDuration.WithLabelValues("delete", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("blob", "delete", i.backend, "error").Inc()
	}
	return err
}

func (i *instrumentedBlobStore) List(ctx context.Context, prefix *string) ([]string, error) {
	start := time.Now()
	list, err := i.inner.List(ctx, prefix)
	i.metrics.BlobOperationDuration.WithLabelValues("list", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("blob", "list", i.backend, "error").Inc()
	}
	return list, err
}

func (i *instrumentedBlobStore) PresignURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	start := time.Now()
	url, err := i.inner.PresignURL(ctx, key, expiry)
	i.metrics.BlobOperationDuration.WithLabelValues("presign_url", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("blob", "presign_url", i.backend, "error").Inc()
	}
	return url, err
}

// --- Instrumented PubSub Store ---

type instrumentedPubSubStore struct {
	inner   PubSubStore
	metrics *Metrics
	backend string
}

func WrapPubSubWithMetrics(inner PubSubStore, metrics *Metrics, backend string) PubSubStore {
	if metrics == nil {
		metrics = DefaultMetrics()
	}
	return &instrumentedPubSubStore{inner: inner, metrics: metrics, backend: backend}
}

func (i *instrumentedPubSubStore) Publish(ctx context.Context, channel string, message []byte) error {
	start := time.Now()
	err := i.inner.Publish(ctx, channel, message)
	i.metrics.PubSubOperationDuration.WithLabelValues("publish", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("pubsub", "publish", i.backend, "error").Inc()
	}
	return err
}

func (i *instrumentedPubSubStore) Subscribe(ctx context.Context, channels []string) (<-chan Message, error) {
	start := time.Now()
	ch, err := i.inner.Subscribe(ctx, channels)
	i.metrics.PubSubOperationDuration.WithLabelValues("subscribe", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("pubsub", "subscribe", i.backend, "error").Inc()
	}
	return ch, err
}

func (i *instrumentedPubSubStore) Unsubscribe(ctx context.Context, channels []string) error {
	start := time.Now()
	err := i.inner.Unsubscribe(ctx, channels)
	i.metrics.PubSubOperationDuration.WithLabelValues("unsubscribe", i.backend).Observe(time.Since(start).Seconds())
	if err != nil {
		i.metrics.ErrorsTotal.WithLabelValues("pubsub", "unsubscribe", i.backend, "error").Inc()
	}
	return err
}
