//! Blob Store Contract
//!
//! Storage for large binary objects. Used for TLS certificates, configuration backups,
//! deployment artifacts, user data exports.

use anyhow::Result;
use async_trait::async_trait;
use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;
use std::time::Duration;

#[derive(Clone, Debug, Default)]
pub struct Metadata {
    pub content_type: Option<String>,
    pub custom: HashMap<String, String>,
}

#[async_trait]
pub trait BlobStore: Send + Sync {
    async fn put(&self, key: &str, data: &[u8], metadata: Option<Metadata>) -> Result<()>;
    async fn get(&self, key: &str) -> Result<Option<(Vec<u8>, Metadata)>>;
    async fn delete(&self, key: &str) -> Result<()>;
    async fn list(&self, prefix: Option<&str>) -> Result<Vec<String>>;
    async fn presign_url(&self, key: &str, expiry: Duration) -> Result<String>;
}

#[derive(Clone)]
struct BlobData {
    data: Vec<u8>,
    metadata: Metadata,
}

/// In-memory implementation of BlobStore for testing and local development.
#[derive(Default)]
pub struct MemoryBlobStore {
    blobs: Arc<RwLock<HashMap<String, BlobData>>>,
}

#[async_trait]
impl BlobStore for MemoryBlobStore {
    async fn put(&self, key: &str, data: &[u8], metadata: Option<Metadata>) -> Result<()> {
        let mut blobs = self.blobs.write().await;
        blobs.insert(
            key.to_string(),
            BlobData {
                data: data.to_vec(),
                metadata: metadata.unwrap_or_default(),
            },
        );
        Ok(())
    }

    async fn get(&self, key: &str) -> Result<Option<(Vec<u8>, Metadata)>> {
        let blobs = self.blobs.read().await;
        Ok(blobs.get(key).map(|b| (b.data.clone(), b.metadata.clone())))
    }

    async fn delete(&self, key: &str) -> Result<()> {
        let mut blobs = self.blobs.write().await;
        blobs.remove(key);
        Ok(())
    }

    async fn list(&self, prefix: Option<&str>) -> Result<Vec<String>> {
        let blobs = self.blobs.read().await;
        let mut keys: Vec<String> = blobs.keys().cloned().collect();
        
        if let Some(p) = prefix {
            keys.retain(|k| k.starts_with(p));
        }
        
        keys.sort();
        Ok(keys)
    }

    async fn presign_url(&self, key: &str, _expiry: Duration) -> Result<String> {
        // In-memory implementation returns a mock URL
        Ok(format!("mock://blob-store/{}", key))
    }
}