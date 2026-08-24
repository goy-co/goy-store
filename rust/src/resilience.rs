//! Resilience module: Retry with exponential backoff and Jitter, Circuit Breaker, and Operation Timeout.

use anyhow::{Result, anyhow};
use async_trait::async_trait;
use std::future::Future;
use std::sync::Arc;
use std::sync::atomic::{AtomicU32, Ordering};
use std::time::{Duration, Instant};
use tokio::sync::RwLock;

use crate::blob::{BlobStore, Metadata};
use crate::config::ResilienceConfig;
use crate::kv::KvStore;
use crate::metrics::StoreMetrics;
use crate::pubsub::{Message, PubSubStore};
use crate::relational::{Migration, Param, RelationalStore, Rows};
use crate::sorted_set::SortedSetStore;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CircuitState {
    Closed,
    Open,
    HalfOpen,
}

pub struct CircuitBreaker {
    state: RwLock<CircuitState>,
    consecutive_failures: AtomicU32,
    failure_threshold: u32,
    reset_timeout: Duration,
    last_failure_time: RwLock<Option<Instant>>,
}

impl CircuitBreaker {
    pub fn new(threshold: u32, reset_timeout: Duration) -> Self {
        Self {
            state: RwLock::new(CircuitState::Closed),
            consecutive_failures: AtomicU32::new(0),
            failure_threshold: threshold,
            reset_timeout,
            last_failure_time: RwLock::new(None),
        }
    }

    pub async fn can_execute(&self) -> Result<()> {
        let mut state_guard = self.state.write().await;
        match *state_guard {
            CircuitState::Closed => Ok(()),
            CircuitState::Open => {
                let last_time = self.last_failure_time.read().await;
                if let Some(instant) = *last_time
                    && instant.elapsed() >= self.reset_timeout
                {
                    *state_guard = CircuitState::HalfOpen;
                    return Ok(());
                }
                Err(anyhow!("Circuit breaker is OPEN"))
            }
            CircuitState::HalfOpen => Ok(()),
        }
    }

    pub async fn on_success(&self) {
        let mut state_guard = self.state.write().await;
        self.consecutive_failures.store(0, Ordering::Relaxed);
        *state_guard = CircuitState::Closed;
    }

    pub async fn on_failure(&self) {
        let failures = self.consecutive_failures.fetch_add(1, Ordering::Relaxed) + 1;
        let mut last_time = self.last_failure_time.write().await;
        *last_time = Some(Instant::now());

        if failures >= self.failure_threshold {
            let mut state_guard = self.state.write().await;
            *state_guard = CircuitState::Open;
        }
    }

    pub async fn current_state_code(&self) -> f64 {
        let state_guard = self.state.read().await;
        match *state_guard {
            CircuitState::Closed => 0.0,
            CircuitState::Open => 1.0,
            CircuitState::HalfOpen => 2.0,
        }
    }
}

pub struct ResilientExecutor {
    cb: Arc<CircuitBreaker>,
    config: ResilienceConfig,
    metrics: Arc<StoreMetrics>,
    contract: String,
    backend: String,
}

impl ResilientExecutor {
    pub fn new(
        config: ResilienceConfig,
        metrics: Arc<StoreMetrics>,
        contract: &str,
        backend: &str,
    ) -> Self {
        let cb = Arc::new(CircuitBreaker::new(
            config.circuit_breaker_threshold,
            Duration::from_secs(config.circuit_breaker_reset_seconds),
        ));
        Self {
            cb,
            config,
            metrics,
            contract: contract.to_string(),
            backend: backend.to_string(),
        }
    }

    pub async fn execute<F, Fut, T>(&self, op_name: &str, operation: F) -> Result<T>
    where
        F: Fn() -> Fut,
        Fut: Future<Output = Result<T>>,
    {
        self.cb.can_execute().await?;

        let timeout_duration = Duration::from_secs(self.config.operation_timeout_seconds);
        let mut attempt = 0;
        let max_retries = self.config.max_retries;
        let base_backoff = Duration::from_millis(self.config.base_backoff_ms);

        loop {
            attempt += 1;
            let fut = operation();
            let res = match tokio::time::timeout(timeout_duration, fut).await {
                Ok(inner_res) => inner_res,
                Err(_) => Err(anyhow!("operation timed out")),
            };

            match res {
                Ok(val) => {
                    self.cb.on_success().await;
                    self.metrics
                        .circuit_breaker_state
                        .with_label_values(&[&self.contract, &self.backend])
                        .set(self.cb.current_state_code().await);
                    return Ok(val);
                }
                Err(err) => {
                    self.cb.on_failure().await;
                    self.metrics
                        .circuit_breaker_state
                        .with_label_values(&[&self.contract, &self.backend])
                        .set(self.cb.current_state_code().await);

                    if attempt > max_retries {
                        return Err(err);
                    }

                    self.metrics
                        .retries_total
                        .with_label_values(&[&self.contract, op_name, &self.backend])
                        .inc();

                    // Calculate exponential backoff with ±25% jitter
                    let factor = 2u64.pow(attempt - 1);
                    let backoff_ms = base_backoff.as_millis() as f64 * factor as f64;
                    // Deterministic or pseudorandom variation between 0.75 and 1.25
                    let jitter = 0.75 + ((attempt as f64 * 137.0) % 50.0) / 100.0;
                    let sleep_duration = Duration::from_millis((backoff_ms * jitter) as u64);
                    tokio::time::sleep(sleep_duration).await;
                }
            }
        }
    }
}

// --- Resilient Wrappers ---

pub struct ResilientKvStore {
    inner: Arc<dyn KvStore>,
    executor: ResilientExecutor,
}

impl ResilientKvStore {
    pub fn new(
        inner: Arc<dyn KvStore>,
        config: ResilienceConfig,
        metrics: Arc<StoreMetrics>,
        backend: &str,
    ) -> Self {
        Self {
            inner,
            executor: ResilientExecutor::new(config, metrics, "kv", backend),
        }
    }
}

#[async_trait]
impl KvStore for ResilientKvStore {
    async fn get(&self, key: &str) -> Result<Option<Vec<u8>>> {
        let k = key.to_string();
        self.executor.execute("get", || self.inner.get(&k)).await
    }

    async fn set(&self, key: &str, value: &[u8], ttl: Option<Duration>) -> Result<()> {
        let k = key.to_string();
        let v = value.to_vec();
        self.executor
            .execute("set", || self.inner.set(&k, &v, ttl))
            .await
    }

    async fn delete(&self, key: &str) -> Result<()> {
        let k = key.to_string();
        self.executor
            .execute("delete", || self.inner.delete(&k))
            .await
    }

    async fn exists(&self, key: &str) -> Result<bool> {
        let k = key.to_string();
        self.executor
            .execute("exists", || self.inner.exists(&k))
            .await
    }

    async fn set_if_not_exists(
        &self,
        key: &str,
        value: &[u8],
        ttl: Option<Duration>,
    ) -> Result<bool> {
        let k = key.to_string();
        let v = value.to_vec();
        self.executor
            .execute("set_if_not_exists", || {
                self.inner.set_if_not_exists(&k, &v, ttl)
            })
            .await
    }

    async fn is_healthy(&self) -> Result<crate::health::HealthStatus> {
        self.inner.is_healthy().await
    }
}

pub struct ResilientRelationalStore {
    inner: Arc<dyn RelationalStore>,
    executor: ResilientExecutor,
}

impl ResilientRelationalStore {
    pub fn new(
        inner: Arc<dyn RelationalStore>,
        config: ResilienceConfig,
        metrics: Arc<StoreMetrics>,
        backend: &str,
    ) -> Self {
        Self {
            inner,
            executor: ResilientExecutor::new(config, metrics, "relational", backend),
        }
    }
}

#[async_trait]
impl RelationalStore for ResilientRelationalStore {
    async fn query(&self, sql: &str, params: &[Param]) -> Result<Rows> {
        let s = sql.to_string();
        let p = params.to_vec();
        self.executor
            .execute("query", || self.inner.query(&s, &p))
            .await
    }

    async fn execute(&self, sql: &str, params: &[Param]) -> Result<u64> {
        let s = sql.to_string();
        let p = params.to_vec();
        self.executor
            .execute("execute", || self.inner.execute(&s, &p))
            .await
    }

    async fn migrate(&self, migrations: &[Migration]) -> Result<()> {
        let m = migrations.to_vec();
        self.executor
            .execute("migrate", || self.inner.migrate(&m))
            .await
    }

    async fn is_healthy(&self) -> Result<crate::health::HealthStatus> {
        self.inner.is_healthy().await
    }
}

pub struct ResilientSortedSetStore {
    inner: Arc<dyn SortedSetStore>,
    executor: ResilientExecutor,
}

impl ResilientSortedSetStore {
    pub fn new(
        inner: Arc<dyn SortedSetStore>,
        config: ResilienceConfig,
        metrics: Arc<StoreMetrics>,
        backend: &str,
    ) -> Self {
        Self {
            inner,
            executor: ResilientExecutor::new(config, metrics, "sorted_set", backend),
        }
    }
}

#[async_trait]
impl SortedSetStore for ResilientSortedSetStore {
    async fn add(&self, set: &str, member: &str, score: f64) -> Result<()> {
        let s = set.to_string();
        let m = member.to_string();
        self.executor
            .execute("add", || self.inner.add(&s, &m, score))
            .await
    }

    async fn remove(&self, set: &str, member: &str) -> Result<()> {
        let s = set.to_string();
        let m = member.to_string();
        self.executor
            .execute("remove", || self.inner.remove(&s, &m))
            .await
    }

    async fn range_by_score(
        &self,
        set: &str,
        min: f64,
        max: f64,
        limit: Option<usize>,
    ) -> Result<Vec<(String, f64)>> {
        let s = set.to_string();
        self.executor
            .execute("range_by_score", || {
                self.inner.range_by_score(&s, min, max, limit)
            })
            .await
    }

    async fn count(&self, set: &str) -> Result<u64> {
        let s = set.to_string();
        self.executor
            .execute("count", || self.inner.count(&s))
            .await
    }

    async fn remove_range(&self, set: &str, min: f64, max: f64) -> Result<u64> {
        let s = set.to_string();
        self.executor
            .execute("remove_range", || self.inner.remove_range(&s, min, max))
            .await
    }

    async fn score(&self, set: &str, member: &str) -> Result<Option<f64>> {
        let s = set.to_string();
        let m = member.to_string();
        self.executor
            .execute("score", || self.inner.score(&s, &m))
            .await
    }

    async fn is_healthy(&self) -> Result<crate::health::HealthStatus> {
        self.inner.is_healthy().await
    }
}

pub struct ResilientBlobStore {
    inner: Arc<dyn BlobStore>,
    executor: ResilientExecutor,
}

impl ResilientBlobStore {
    pub fn new(
        inner: Arc<dyn BlobStore>,
        config: ResilienceConfig,
        metrics: Arc<StoreMetrics>,
        backend: &str,
    ) -> Self {
        Self {
            inner,
            executor: ResilientExecutor::new(config, metrics, "blob", backend),
        }
    }
}

#[async_trait]
impl BlobStore for ResilientBlobStore {
    async fn put(&self, key: &str, data: &[u8], metadata: Option<Metadata>) -> Result<()> {
        let k = key.to_string();
        let d = data.to_vec();
        let m = metadata.clone();
        self.executor
            .execute("put", || self.inner.put(&k, &d, m.clone()))
            .await
    }

    async fn get(&self, key: &str) -> Result<Option<(Vec<u8>, Metadata)>> {
        let k = key.to_string();
        self.executor.execute("get", || self.inner.get(&k)).await
    }

    async fn delete(&self, key: &str) -> Result<()> {
        let k = key.to_string();
        self.executor
            .execute("delete", || self.inner.delete(&k))
            .await
    }

    async fn list(&self, prefix: Option<&str>) -> Result<Vec<String>> {
        let p = prefix.map(|s| s.to_string());
        self.executor
            .execute("list", || self.inner.list(p.as_deref()))
            .await
    }

    async fn presign_url(&self, key: &str, expiry: Duration) -> Result<String> {
        let k = key.to_string();
        self.executor
            .execute("presign_url", || self.inner.presign_url(&k, expiry))
            .await
    }

    async fn is_healthy(&self) -> Result<crate::health::HealthStatus> {
        self.inner.is_healthy().await
    }
}

pub struct ResilientPubSubStore {
    inner: Arc<dyn PubSubStore>,
    executor: ResilientExecutor,
}

impl ResilientPubSubStore {
    pub fn new(
        inner: Arc<dyn PubSubStore>,
        config: ResilienceConfig,
        metrics: Arc<StoreMetrics>,
        backend: &str,
    ) -> Self {
        Self {
            inner,
            executor: ResilientExecutor::new(config, metrics, "pubsub", backend),
        }
    }
}

#[async_trait]
impl PubSubStore for ResilientPubSubStore {
    async fn publish(&self, channel: &str, message: &[u8]) -> Result<()> {
        let c = channel.to_string();
        let m = message.to_vec();
        self.executor
            .execute("publish", || self.inner.publish(&c, &m))
            .await
    }

    async fn subscribe(
        &self,
        channels: &[&str],
    ) -> Result<tokio_stream::wrappers::BroadcastStream<Message>> {
        self.inner.subscribe(channels).await
    }

    async fn unsubscribe(&self, channels: &[&str]) -> Result<()> {
        let c: Vec<String> = channels.iter().map(|s| s.to_string()).collect();
        let c_refs: Vec<&str> = c.iter().map(|s| s.as_str()).collect();
        self.inner.unsubscribe(&c_refs).await
    }

    async fn is_healthy(&self) -> Result<crate::health::HealthStatus> {
        self.inner.is_healthy().await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use prometheus::Registry;
    use std::sync::atomic::AtomicBool;

    #[tokio::test]
    async fn test_circuit_breaker_transitions() {
        let cb = CircuitBreaker::new(2, Duration::from_millis(50));
        assert!(cb.can_execute().await.is_ok());

        cb.on_failure().await;
        assert!(cb.can_execute().await.is_ok());

        cb.on_failure().await;
        // Now open
        assert!(cb.can_execute().await.is_err());

        tokio::time::sleep(Duration::from_millis(60)).await;
        // After reset timeout -> half-open
        assert!(cb.can_execute().await.is_ok());

        // Success resets to closed
        cb.on_success().await;
        assert!(cb.can_execute().await.is_ok());
    }

    #[tokio::test]
    async fn test_resilient_executor_retry() {
        let registry = Registry::new();
        let metrics = Arc::new(StoreMetrics::new(&registry).unwrap());
        let config = ResilienceConfig {
            max_retries: 2,
            base_backoff_ms: 10,
            circuit_breaker_threshold: 5,
            circuit_breaker_reset_seconds: 10,
            operation_timeout_seconds: 2,
        };
        let exec = ResilientExecutor::new(config, metrics.clone(), "test", "mem");

        let failed_once = Arc::new(AtomicBool::new(false));
        let failed_clone = failed_once.clone();

        let res = exec
            .execute("test_op", || {
                let f = failed_clone.clone();
                async move {
                    if !f.swap(true, Ordering::SeqCst) {
                        Err(anyhow!("transient error"))
                    } else {
                        Ok(42)
                    }
                }
            })
            .await;

        assert_eq!(res.unwrap(), 42);
        assert_eq!(
            metrics
                .retries_total
                .with_label_values(&["test", "test_op", "mem"])
                .get(),
            1.0
        );
    }
}
