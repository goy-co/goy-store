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
        // In a real implementation, this would instantiate the appropriate
        // backend implementations based on the config.
        // For now, we return a memory-based implementation for all contracts.
        
        Ok(Self {
            kv: Arc::new(kv::MemoryKvStore::default()),
            relational: Arc::new(relational::MemoryRelationalStore::default()),
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