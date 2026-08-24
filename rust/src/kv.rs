//! Key-Value Store Contract
//!
//! Basic key-value operations with optional TTL. Used for sessions, caches, tokens, temporary state.

use anyhow::Result;
use std::time::Duration;
use async_trait::async_trait;
use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;

#[async_trait]
pub trait KvStore: Send + Sync {
    async fn get(&self, key: &str) -> Result<Option<Vec<u8>>>;
    async fn set(&self, key: &str, value: &[u8], ttl: Option<Duration>) -> Result<()>;
    async fn delete(&self, key: &str) -> Result<()>;
    async fn exists(&self, key: &str) -> Result<bool>;
    async fn set_if_not_exists(&self, key: &str, value: &[u8], ttl: Option<Duration>) -> Result<bool>;
}

/// In-memory implementation of KvStore for testing and local development.
#[derive(Default)]
pub struct MemoryKvStore {
    data: Arc<RwLock<HashMap<String, Vec<u8>>>>,
}

#[async_trait]
impl KvStore for MemoryKvStore {
    async fn get(&self, key: &str) -> Result<Option<Vec<u8>>> {
        let data = self.data.read().await;
        Ok(data.get(key).cloned())
    }

    async fn set(&self, key: &str, value: &[u8], _ttl: Option<Duration>) -> Result<()> {
        let mut data = self.data.write().await;
        data.insert(key.to_string(), value.to_vec());
        Ok(())
    }

    async fn delete(&self, key: &str) -> Result<()> {
        let mut data = self.data.write().await;
        data.remove(key);
        Ok(())
    }

    async fn exists(&self, key: &str) -> Result<bool> {
        let data = self.data.read().await;
        Ok(data.contains_key(key))
    }

    async fn set_if_not_exists(&self, key: &str, value: &[u8], _ttl: Option<Duration>) -> Result<bool> {
        let mut data = self.data.write().await;
        if data.contains_key(key) {
            Ok(false)
        } else {
            data.insert(key.to_string(), value.to_vec());
            Ok(true)
        }
    }
}