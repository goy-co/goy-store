//! Goy Store: Unified Persistence Abstraction
//!
//! Goy Store provides a consistent API for all persistence operations,
//! regardless of the underlying backend. It manages connections efficiently,
//! guarantees resilience, and exposes native observability.

pub mod kv;
pub mod relational;
pub mod sorted_set;
pub mod pubsub;
pub mod blob;
pub mod config;
pub mod metrics;

pub use kv::KvStore;
pub use relational::RelationalStore;
pub use sorted_set::SortedSetStore;
pub use pubsub::PubSubStore;
pub use blob::BlobStore;
pub use config::StoreConfig;

pub use metrics::StoreMetrics;

use std::sync::Arc;
use anyhow::Result;

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
    pub async fn from_config_with_metrics(config: &StoreConfig, metrics: Arc<StoreMetrics>) -> Result<Self> {
        #[cfg(feature = "redis-backend")]
        let (redis_conn, redis_client): (Option<redis::aio::ConnectionManager>, Option<redis::Client>) = {
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
        let kv = Arc::new(metrics::InstrumentedKvStore::new(
            kv_raw,
            metrics.clone(),
            &config.kv.backend,
        ));

        let sorted_set_raw: Arc<dyn SortedSetStore> = match config.sorted_set.backend.as_str() {
            #[cfg(feature = "redis-backend")]
            "redis" => {
                let conn = redis_conn.clone().expect("redis connection manager exists");
                Arc::new(sorted_set::RedisSortedSetStore::from_connection_manager(conn))
            }
            _ => Arc::new(sorted_set::MemorySortedSetStore::default()),
        };
        let sorted_set = Arc::new(metrics::InstrumentedSortedSetStore::new(
            sorted_set_raw,
            metrics.clone(),
            &config.sorted_set.backend,
        ));

        let pubsub_raw: Arc<dyn PubSubStore> = match config.pubsub.backend.as_str() {
            #[cfg(feature = "redis-backend")]
            "redis" => {
                let conn = redis_conn.expect("redis connection manager exists");
                let client = redis_client.expect("redis client exists");
                Arc::new(pubsub::RedisPubSubStore::from_connection_manager(conn, client))
            }
            _ => Arc::new(pubsub::MemoryPubSubStore::default()),
        };
        let pubsub = Arc::new(metrics::InstrumentedPubSubStore::new(
            pubsub_raw,
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
            _ => Arc::new(relational::MemoryRelationalStore::default()),
        };
        let relational = Arc::new(metrics::InstrumentedRelationalStore::new(
            relational_raw,
            metrics.clone(),
            &config.relational.backend,
        ));

        let blob_raw: Arc<dyn BlobStore> = match config.blob.backend.as_str() {
            "local" | "filesystem" => {
                let path = config.blob.path.as_deref().unwrap_or("./data/blobs");
                Arc::new(blob::LocalBlobStore::new(path))
            }
            _ => Arc::new(blob::MemoryBlobStore::default()),
        };
        let blob = Arc::new(metrics::InstrumentedBlobStore::new(
            blob_raw,
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
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_store_creation() {
        let config = StoreConfig::default();
        let store = GoyStore::from_config(&config).await.unwrap();
        
        // Test KV store
        store.kv.set("test_key", b"test_value", None).await.unwrap();
        let result = store.kv.get("test_key").await.unwrap();
        assert_eq!(result, Some(b"test_value".to_vec()));
    }
}