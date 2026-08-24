//! Native observability for Goy Store
//!
//! Every Goy Store operation emits metrics without additional code in consumer services.

use anyhow::Result;
use async_trait::async_trait;
use prometheus::{
    CounterVec, GaugeVec, HistogramOpts, HistogramVec, Opts, Registry,
    register_counter_vec_with_registry, register_gauge_vec_with_registry,
    register_histogram_vec_with_registry,
};
use std::sync::Arc;
use std::time::{Duration, Instant};

use crate::blob::{BlobStore, Metadata};
use crate::kv::KvStore;
use crate::pubsub::{Message, PubSubStore};
use crate::relational::{Migration, Param, RelationalStore, Rows};
use crate::sorted_set::SortedSetStore;

pub struct StoreMetrics {
    pub kv_operation_duration_seconds: HistogramVec,
    pub relational_query_duration_seconds: HistogramVec,
    pub sorted_set_operation_duration_seconds: HistogramVec,
    pub blob_operation_duration_seconds: HistogramVec,
    pub pubsub_operation_duration_seconds: HistogramVec,
    pub errors_total: CounterVec,
    pub retries_total: CounterVec,
    pub circuit_breaker_state: GaugeVec,
    pub pool_active_connections: GaugeVec,
    pub pool_idle_connections: GaugeVec,
    pub health_check_status: GaugeVec,
    pub health_check_duration_seconds: HistogramVec,
}

impl StoreMetrics {
    pub fn new(registry: &Registry) -> Result<Self> {
        let kv_operation_duration_seconds = register_histogram_vec_with_registry!(
            HistogramOpts::new(
                "goy_store_kv_operation_duration_seconds",
                "Duration of KV store operations in seconds"
            ),
            &["operation", "backend"],
            registry
        )?;

        let relational_query_duration_seconds = register_histogram_vec_with_registry!(
            HistogramOpts::new(
                "goy_store_relational_query_duration_seconds",
                "Duration of relational query operations in seconds"
            ),
            &["operation", "backend"],
            registry
        )?;

        let sorted_set_operation_duration_seconds = register_histogram_vec_with_registry!(
            HistogramOpts::new(
                "goy_store_sorted_set_operation_duration_seconds",
                "Duration of sorted set operations in seconds"
            ),
            &["operation", "backend"],
            registry
        )?;

        let blob_operation_duration_seconds = register_histogram_vec_with_registry!(
            HistogramOpts::new(
                "goy_store_blob_operation_duration_seconds",
                "Duration of blob operations in seconds"
            ),
            &["operation", "backend"],
            registry
        )?;

        let pubsub_operation_duration_seconds = register_histogram_vec_with_registry!(
            HistogramOpts::new(
                "goy_store_pubsub_operation_duration_seconds",
                "Duration of pubsub operations in seconds"
            ),
            &["operation", "backend"],
            registry
        )?;

        let errors_total = register_counter_vec_with_registry!(
            Opts::new("goy_store_errors_total", "Total number of store errors"),
            &["contract", "operation", "backend", "error_type"],
            registry
        )?;

        let retries_total = register_counter_vec_with_registry!(
            Opts::new(
                "goy_store_retries_total",
                "Total number of store operation retries"
            ),
            &["contract", "operation", "backend"],
            registry
        )?;

        let circuit_breaker_state = register_gauge_vec_with_registry!(
            Opts::new(
                "goy_store_circuit_breaker_state",
                "Circuit breaker state (0=closed, 1=open, 2=half-open)"
            ),
            &["contract", "backend"],
            registry
        )?;

        let pool_active_connections = register_gauge_vec_with_registry!(
            Opts::new(
                "goy_store_pool_active_connections",
                "Number of active pool connections"
            ),
            &["backend"],
            registry
        )?;

        let pool_idle_connections = register_gauge_vec_with_registry!(
            Opts::new(
                "goy_store_pool_idle_connections",
                "Number of idle pool connections"
            ),
            &["backend"],
            registry
        )?;

        let health_check_status = register_gauge_vec_with_registry!(
            Opts::new(
                "goy_store_health_check_status",
                "Health check status (1=healthy, 0=unhealthy, 2=degraded)"
            ),
            &["contract", "backend"],
            registry
        )?;

        let health_check_duration_seconds = register_histogram_vec_with_registry!(
            HistogramOpts::new(
                "goy_store_health_check_duration_seconds",
                "Duration of health check operations in seconds"
            ),
            &["contract", "backend"],
            registry
        )?;

        Ok(Self {
            kv_operation_duration_seconds,
            relational_query_duration_seconds,
            sorted_set_operation_duration_seconds,
            blob_operation_duration_seconds,
            pubsub_operation_duration_seconds,
            errors_total,
            retries_total,
            circuit_breaker_state,
            pool_active_connections,
            pool_idle_connections,
            health_check_status,
            health_check_duration_seconds,
        })
    }
}

impl Default for StoreMetrics {
    fn default() -> Self {
        let registry = Registry::new();
        Self::new(&registry).expect("Failed to create default metrics")
    }
}

// --- Instrumented Wrappers ---

pub struct InstrumentedKvStore {
    inner: Arc<dyn KvStore>,
    metrics: Arc<StoreMetrics>,
    backend: String,
}

impl InstrumentedKvStore {
    pub fn new(inner: Arc<dyn KvStore>, metrics: Arc<StoreMetrics>, backend: &str) -> Self {
        Self {
            inner,
            metrics,
            backend: backend.to_string(),
        }
    }
}

#[async_trait]
impl KvStore for InstrumentedKvStore {
    async fn get(&self, key: &str) -> Result<Option<Vec<u8>>> {
        let start = Instant::now();
        let res = self.inner.get(key).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .kv_operation_duration_seconds
            .with_label_values(&["get", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["kv", "get", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn set(&self, key: &str, value: &[u8], ttl: Option<Duration>) -> Result<()> {
        let start = Instant::now();
        let res = self.inner.set(key, value, ttl).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .kv_operation_duration_seconds
            .with_label_values(&["set", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["kv", "set", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn delete(&self, key: &str) -> Result<()> {
        let start = Instant::now();
        let res = self.inner.delete(key).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .kv_operation_duration_seconds
            .with_label_values(&["delete", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["kv", "delete", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn exists(&self, key: &str) -> Result<bool> {
        let start = Instant::now();
        let res = self.inner.exists(key).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .kv_operation_duration_seconds
            .with_label_values(&["exists", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["kv", "exists", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn set_if_not_exists(
        &self,
        key: &str,
        value: &[u8],
        ttl: Option<Duration>,
    ) -> Result<bool> {
        let start = Instant::now();
        let res = self.inner.set_if_not_exists(key, value, ttl).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .kv_operation_duration_seconds
            .with_label_values(&["set_if_not_exists", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["kv", "set_if_not_exists", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn is_healthy(&self) -> Result<crate::health::HealthStatus> {
        let start = Instant::now();
        let status = self.inner.is_healthy().await?;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .health_check_duration_seconds
            .with_label_values(&["kv", &self.backend])
            .observe(duration);
        let val = match status.state {
            crate::health::HealthState::Healthy => 1.0,
            crate::health::HealthState::Degraded => 2.0,
            crate::health::HealthState::Unhealthy => 0.0,
        };
        self.metrics
            .health_check_status
            .with_label_values(&["kv", &self.backend])
            .set(val);
        Ok(status)
    }
}

pub struct InstrumentedRelationalStore {
    inner: Arc<dyn RelationalStore>,
    metrics: Arc<StoreMetrics>,
    backend: String,
}

impl InstrumentedRelationalStore {
    pub fn new(inner: Arc<dyn RelationalStore>, metrics: Arc<StoreMetrics>, backend: &str) -> Self {
        Self {
            inner,
            metrics,
            backend: backend.to_string(),
        }
    }
}

#[async_trait]
impl RelationalStore for InstrumentedRelationalStore {
    async fn query(&self, sql: &str, params: &[Param]) -> Result<Rows> {
        let start = Instant::now();
        let res = self.inner.query(sql, params).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .relational_query_duration_seconds
            .with_label_values(&["query", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["relational", "query", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn execute(&self, sql: &str, params: &[Param]) -> Result<u64> {
        let start = Instant::now();
        let res = self.inner.execute(sql, params).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .relational_query_duration_seconds
            .with_label_values(&["execute", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["relational", "execute", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn migrate(&self, migrations: &[Migration]) -> Result<()> {
        let start = Instant::now();
        let res = self.inner.migrate(migrations).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .relational_query_duration_seconds
            .with_label_values(&["migrate", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["relational", "migrate", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn is_healthy(&self) -> Result<crate::health::HealthStatus> {
        let start = Instant::now();
        let status = self.inner.is_healthy().await?;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .health_check_duration_seconds
            .with_label_values(&["relational", &self.backend])
            .observe(duration);
        let val = match status.state {
            crate::health::HealthState::Healthy => 1.0,
            crate::health::HealthState::Degraded => 2.0,
            crate::health::HealthState::Unhealthy => 0.0,
        };
        self.metrics
            .health_check_status
            .with_label_values(&["relational", &self.backend])
            .set(val);
        Ok(status)
    }
}

pub struct InstrumentedSortedSetStore {
    inner: Arc<dyn SortedSetStore>,
    metrics: Arc<StoreMetrics>,
    backend: String,
}

impl InstrumentedSortedSetStore {
    pub fn new(inner: Arc<dyn SortedSetStore>, metrics: Arc<StoreMetrics>, backend: &str) -> Self {
        Self {
            inner,
            metrics,
            backend: backend.to_string(),
        }
    }
}

#[async_trait]
impl SortedSetStore for InstrumentedSortedSetStore {
    async fn add(&self, set: &str, member: &str, score: f64) -> Result<()> {
        let start = Instant::now();
        let res = self.inner.add(set, member, score).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .sorted_set_operation_duration_seconds
            .with_label_values(&["add", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["sorted_set", "add", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn remove(&self, set: &str, member: &str) -> Result<()> {
        let start = Instant::now();
        let res = self.inner.remove(set, member).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .sorted_set_operation_duration_seconds
            .with_label_values(&["remove", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["sorted_set", "remove", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn range_by_score(
        &self,
        set: &str,
        min: f64,
        max: f64,
        limit: Option<usize>,
    ) -> Result<Vec<(String, f64)>> {
        let start = Instant::now();
        let res = self.inner.range_by_score(set, min, max, limit).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .sorted_set_operation_duration_seconds
            .with_label_values(&["range_by_score", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["sorted_set", "range_by_score", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn count(&self, set: &str) -> Result<u64> {
        let start = Instant::now();
        let res = self.inner.count(set).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .sorted_set_operation_duration_seconds
            .with_label_values(&["count", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["sorted_set", "count", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn remove_range(&self, set: &str, min: f64, max: f64) -> Result<u64> {
        let start = Instant::now();
        let res = self.inner.remove_range(set, min, max).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .sorted_set_operation_duration_seconds
            .with_label_values(&["remove_range", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["sorted_set", "remove_range", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn score(&self, set: &str, member: &str) -> Result<Option<f64>> {
        let start = Instant::now();
        let res = self.inner.score(set, member).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .sorted_set_operation_duration_seconds
            .with_label_values(&["score", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["sorted_set", "score", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn is_healthy(&self) -> Result<crate::health::HealthStatus> {
        let start = Instant::now();
        let status = self.inner.is_healthy().await?;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .health_check_duration_seconds
            .with_label_values(&["sorted_set", &self.backend])
            .observe(duration);
        let val = match status.state {
            crate::health::HealthState::Healthy => 1.0,
            crate::health::HealthState::Degraded => 2.0,
            crate::health::HealthState::Unhealthy => 0.0,
        };
        self.metrics
            .health_check_status
            .with_label_values(&["sorted_set", &self.backend])
            .set(val);
        Ok(status)
    }
}

pub struct InstrumentedBlobStore {
    inner: Arc<dyn BlobStore>,
    metrics: Arc<StoreMetrics>,
    backend: String,
}

impl InstrumentedBlobStore {
    pub fn new(inner: Arc<dyn BlobStore>, metrics: Arc<StoreMetrics>, backend: &str) -> Self {
        Self {
            inner,
            metrics,
            backend: backend.to_string(),
        }
    }
}

#[async_trait]
impl BlobStore for InstrumentedBlobStore {
    async fn put(&self, key: &str, data: &[u8], metadata: Option<Metadata>) -> Result<()> {
        let start = Instant::now();
        let res = self.inner.put(key, data, metadata).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .blob_operation_duration_seconds
            .with_label_values(&["put", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["blob", "put", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn get(&self, key: &str) -> Result<Option<(Vec<u8>, Metadata)>> {
        let start = Instant::now();
        let res = self.inner.get(key).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .blob_operation_duration_seconds
            .with_label_values(&["get", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["blob", "get", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn delete(&self, key: &str) -> Result<()> {
        let start = Instant::now();
        let res = self.inner.delete(key).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .blob_operation_duration_seconds
            .with_label_values(&["delete", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["blob", "delete", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn list(&self, prefix: Option<&str>) -> Result<Vec<String>> {
        let start = Instant::now();
        let res = self.inner.list(prefix).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .blob_operation_duration_seconds
            .with_label_values(&["list", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["blob", "list", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn presign_url(&self, key: &str, expiry: Duration) -> Result<String> {
        let start = Instant::now();
        let res = self.inner.presign_url(key, expiry).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .blob_operation_duration_seconds
            .with_label_values(&["presign_url", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["blob", "presign_url", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn is_healthy(&self) -> Result<crate::health::HealthStatus> {
        let start = Instant::now();
        let status = self.inner.is_healthy().await?;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .health_check_duration_seconds
            .with_label_values(&["blob", &self.backend])
            .observe(duration);
        let val = match status.state {
            crate::health::HealthState::Healthy => 1.0,
            crate::health::HealthState::Degraded => 2.0,
            crate::health::HealthState::Unhealthy => 0.0,
        };
        self.metrics
            .health_check_status
            .with_label_values(&["blob", &self.backend])
            .set(val);
        Ok(status)
    }
}

pub struct InstrumentedPubSubStore {
    inner: Arc<dyn PubSubStore>,
    metrics: Arc<StoreMetrics>,
    backend: String,
}

impl InstrumentedPubSubStore {
    pub fn new(inner: Arc<dyn PubSubStore>, metrics: Arc<StoreMetrics>, backend: &str) -> Self {
        Self {
            inner,
            metrics,
            backend: backend.to_string(),
        }
    }
}

#[async_trait]
impl PubSubStore for InstrumentedPubSubStore {
    async fn publish(&self, channel: &str, message: &[u8]) -> Result<()> {
        let start = Instant::now();
        let res = self.inner.publish(channel, message).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .pubsub_operation_duration_seconds
            .with_label_values(&["publish", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["pubsub", "publish", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn subscribe(
        &self,
        channels: &[&str],
    ) -> Result<tokio_stream::wrappers::BroadcastStream<Message>> {
        let start = Instant::now();
        let res = self.inner.subscribe(channels).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .pubsub_operation_duration_seconds
            .with_label_values(&["subscribe", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["pubsub", "subscribe", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn unsubscribe(&self, channels: &[&str]) -> Result<()> {
        let start = Instant::now();
        let res = self.inner.unsubscribe(channels).await;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .pubsub_operation_duration_seconds
            .with_label_values(&["unsubscribe", &self.backend])
            .observe(duration);
        if res.is_err() {
            self.metrics
                .errors_total
                .with_label_values(&["pubsub", "unsubscribe", &self.backend, "error"])
                .inc();
        }
        res
    }

    async fn is_healthy(&self) -> Result<crate::health::HealthStatus> {
        let start = Instant::now();
        let status = self.inner.is_healthy().await?;
        let duration = start.elapsed().as_secs_f64();
        self.metrics
            .health_check_duration_seconds
            .with_label_values(&["pubsub", &self.backend])
            .observe(duration);
        let val = match status.state {
            crate::health::HealthState::Healthy => 1.0,
            crate::health::HealthState::Degraded => 2.0,
            crate::health::HealthState::Unhealthy => 0.0,
        };
        self.metrics
            .health_check_status
            .with_label_values(&["pubsub", &self.backend])
            .set(val);
        Ok(status)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::kv::MemoryKvStore;

    #[tokio::test]
    async fn test_instrumented_kv_store_metrics() {
        let registry = Registry::new();
        let metrics = Arc::new(StoreMetrics::new(&registry).unwrap());
        let store = Arc::new(MemoryKvStore::default());
        let instrumented = InstrumentedKvStore::new(store, metrics.clone(), "memory");

        instrumented
            .set("test_key", b"test_val", None)
            .await
            .unwrap();
        let val = instrumented.get("test_key").await.unwrap();
        assert_eq!(val, Some(b"test_val".to_vec()));

        let metric_families = registry.gather();
        let kv_duration = metric_families
            .iter()
            .find(|mf| mf.get_name() == "goy_store_kv_operation_duration_seconds");
        assert!(kv_duration.is_some());
    }
}
