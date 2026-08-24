//! Goy Store: Unified Persistence Abstraction
//!
//! Goy Store provides a consistent API for all persistence operations,
//! regardless of the underlying backend. It manages connections efficiently,
//! guarantees resilience, and exposes native observability.

pub mod blob;
pub mod config;
pub mod health;
pub mod kv;
pub mod metrics;
pub mod pubsub;
pub mod relational;
pub mod resilience;
pub mod sorted_set;

pub use blob::BlobStore;
pub use config::StoreConfig;
pub use kv::KvStore;
pub use pubsub::PubSubStore;
pub use relational::RelationalStore;
pub use sorted_set::SortedSetStore;

pub use health::{ConsolidatedHealth, HealthState, HealthStatus};
pub use metrics::StoreMetrics;
pub use resilience::{
    CircuitBreaker, ResilientBlobStore, ResilientKvStore, ResilientPubSubStore,
    ResilientRelationalStore, ResilientSortedSetStore,
};

use anyhow::Result;
use std::sync::Arc;

/// The main Goy Store interface, aggregating all persistence contracts.
pub struct GoyStore {
    pub kv: Arc<dyn KvStore>,
    pub relational: Arc<dyn RelationalStore>,
    pub sorted_set: Arc<dyn SortedSetStore>,
    pub pubsub: Arc<dyn PubSubStore>,
    pub blob: Arc<dyn BlobStore>,
    pub metrics: Arc<StoreMetrics>,
}

impl GoyStore {
    /// Creates a new GoyStore instance from the provided configuration.
    pub async fn from_config(config: &StoreConfig) -> Result<Self> {
        let metrics = Arc::new(StoreMetrics::default());
        Self::from_config_with_metrics(config, metrics).await
    }

    /// Creates a new GoyStore instance with a specific StoreMetrics registry.
    pub async fn from_config_with_metrics(
        config: &StoreConfig,
        metrics: Arc<StoreMetrics>,
    ) -> Result<Self> {
        #[cfg(feature = "redis-backend")]
        let (redis_conn, redis_client): (
            Option<redis::aio::ConnectionManager>,
            Option<redis::Client>,
        ) = {
            if config.kv.backend == "redis"
                || config.sorted_set.backend == "redis"
                || config.pubsub.backend == "redis"
            {
                let url = config
                    .kv
                    .url
                    .as_deref()
                    .or(config.sorted_set.url.as_deref())
                    .or(config.pubsub.url.as_deref())
                    .unwrap_or("redis://127.0.0.1:6379");
                let client = redis::Client::open(url)?;
                let conn = redis::aio::ConnectionManager::new(client.clone()).await?;
                (Some(conn), Some(client))
            } else {
                (None, None)
            }
        };

        let kv_raw: Arc<dyn KvStore> = match config.kv.backend.as_str() {
            #[cfg(feature = "redis-backend")]
            "redis" => {
                let conn = redis_conn.clone().expect("redis connection manager exists");
                Arc::new(kv::RedisKvStore::from_connection_manager(conn))
            }
            _ => Arc::new(kv::MemoryKvStore::default()),
        };
        let kv_instrumented = Arc::new(metrics::InstrumentedKvStore::new(
            kv_raw,
            metrics.clone(),
            &config.kv.backend,
        ));
        let kv = Arc::new(resilience::ResilientKvStore::new(
            kv_instrumented,
            config.resilience.clone(),
            metrics.clone(),
            &config.kv.backend,
        ));

        let sorted_set_raw: Arc<dyn SortedSetStore> = match config.sorted_set.backend.as_str() {
            #[cfg(feature = "redis-backend")]
            "redis" => {
                let conn = redis_conn.clone().expect("redis connection manager exists");
                Arc::new(sorted_set::RedisSortedSetStore::from_connection_manager(
                    conn,
                ))
            }
            _ => Arc::new(sorted_set::MemorySortedSetStore::default()),
        };
        let sorted_set_instrumented = Arc::new(metrics::InstrumentedSortedSetStore::new(
            sorted_set_raw,
            metrics.clone(),
            &config.sorted_set.backend,
        ));
        let sorted_set = Arc::new(resilience::ResilientSortedSetStore::new(
            sorted_set_instrumented,
            config.resilience.clone(),
            metrics.clone(),
            &config.sorted_set.backend,
        ));

        let pubsub_raw: Arc<dyn PubSubStore> = match config.pubsub.backend.as_str() {
            #[cfg(feature = "redis-backend")]
            "redis" => {
                let conn = redis_conn.expect("redis connection manager exists");
                let client = redis_client.expect("redis client exists");
                Arc::new(pubsub::RedisPubSubStore::from_connection_manager(
                    conn, client,
                ))
            }
            _ => Arc::new(pubsub::MemoryPubSubStore::default()),
        };
        let pubsub_instrumented = Arc::new(metrics::InstrumentedPubSubStore::new(
            pubsub_raw,
            metrics.clone(),
            &config.pubsub.backend,
        ));
        let pubsub = Arc::new(resilience::ResilientPubSubStore::new(
            pubsub_instrumented,
            config.resilience.clone(),
            metrics.clone(),
            &config.pubsub.backend,
        ));

        let relational_raw: Arc<dyn RelationalStore> = match config.relational.backend.as_str() {
            #[cfg(feature = "sqlx-backend")]
            "postgres" => {
                let url = config
                    .relational
                    .url
                    .as_deref()
                    .unwrap_or("postgres://postgres:postgres@127.0.0.1:5432/goy");
                Arc::new(relational::PostgresRelationalStore::new(url).await?)
            }
            _ => Arc::new(relational::MemoryRelationalStore),
        };
        let relational_instrumented = Arc::new(metrics::InstrumentedRelationalStore::new(
            relational_raw,
            metrics.clone(),
            &config.relational.backend,
        ));
        let relational = Arc::new(resilience::ResilientRelationalStore::new(
            relational_instrumented,
            config.resilience.clone(),
            metrics.clone(),
            &config.relational.backend,
        ));

        let blob_raw: Arc<dyn BlobStore> = match config.blob.backend.as_str() {
            "local" | "filesystem" => {
                let path = config.blob.path.as_deref().unwrap_or("./data/blobs");
                Arc::new(blob::LocalBlobStore::new(path))
            }
            #[cfg(feature = "s3-backend")]
            "s3" | "minio" | "r2" => Arc::new(blob::S3BlobStore::new(&config.blob).await?),
            _ => Arc::new(blob::MemoryBlobStore::default()),
        };
        let blob_instrumented = Arc::new(metrics::InstrumentedBlobStore::new(
            blob_raw,
            metrics.clone(),
            &config.blob.backend,
        ));
        let blob = Arc::new(resilience::ResilientBlobStore::new(
            blob_instrumented,
            config.resilience.clone(),
            metrics.clone(),
            &config.blob.backend,
        ));

        Ok(Self {
            kv,
            relational,
            sorted_set,
            pubsub,
            blob,
            metrics,
        })
    }

    /// Checks the health of all configured persistence contracts.
    pub async fn health_check(&self) -> ConsolidatedHealth {
        let (kv_res, rel_res, ss_res, ps_res, blob_res) = tokio::join!(
            self.kv.is_healthy(),
            self.relational.is_healthy(),
            self.sorted_set.is_healthy(),
            self.pubsub.is_healthy(),
            self.blob.is_healthy(),
        );

        let statuses = vec![
            kv_res.unwrap_or_else(|e| HealthStatus::unhealthy("kv", "unknown", &e.to_string(), 0)),
            rel_res.unwrap_or_else(|e| {
                HealthStatus::unhealthy("relational", "unknown", &e.to_string(), 0)
            }),
            ss_res.unwrap_or_else(|e| {
                HealthStatus::unhealthy("sorted_set", "unknown", &e.to_string(), 0)
            }),
            ps_res.unwrap_or_else(|e| {
                HealthStatus::unhealthy("pubsub", "unknown", &e.to_string(), 0)
            }),
            blob_res
                .unwrap_or_else(|e| HealthStatus::unhealthy("blob", "unknown", &e.to_string(), 0)),
        ];

        ConsolidatedHealth::from_statuses(statuses)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_store_creation_and_health_check() {
        let config = StoreConfig::default();
        let store = GoyStore::from_config(&config).await.unwrap();

        // Test KV store
        store.kv.set("test_key", b"test_value", None).await.unwrap();
        let result = store.kv.get("test_key").await.unwrap();
        assert_eq!(result, Some(b"test_value".to_vec()));

        // Test Health Check
        let health = store.health_check().await;
        assert_eq!(health.state, HealthState::Healthy);
        assert_eq!(health.contracts.len(), 5);
    }
}
