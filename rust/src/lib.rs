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

use std::sync::Arc;
use anyhow::Result;

/// The main Goy Store interface, aggregating all persistence contracts.
pub struct GoyStore {
    pub kv: Arc<dyn KvStore>,
    pub relational: Arc<dyn RelationalStore>,
    pub sorted_set: Arc<dyn SortedSetStore>,
    pub pubsub: Arc<dyn PubSubStore>,
    pub blob: Arc<dyn BlobStore>,
}

impl GoyStore {
    /// Creates a new GoyStore instance from the provided configuration.
    pub async fn from_config(config: &StoreConfig) -> Result<Self> {
        let kv: Arc<dyn KvStore> = match config.kv.backend.as_str() {
            #[cfg(feature = "redis-backend")]
            "redis" => {
                let url = config.kv.url.as_deref().unwrap_or("redis://127.0.0.1:6379");
                Arc::new(kv::RedisKvStore::new(url).await?)
            }
            _ => Arc::new(kv::MemoryKvStore::default()),
        };

        let relational: Arc<dyn RelationalStore> = match config.relational.backend.as_str() {
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

        Ok(Self {
            kv,
            relational,
            sorted_set: Arc::new(sorted_set::MemorySortedSetStore::default()),
            pubsub: Arc::new(pubsub::MemoryPubSubStore::default()),
            blob: Arc::new(blob::MemoryBlobStore::default()),
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