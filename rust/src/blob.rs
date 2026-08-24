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

use serde::{Deserialize, Serialize};

#[derive(Clone, Debug, Default, Serialize, Deserialize)]
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
    async fn is_healthy(&self) -> Result<crate::health::HealthStatus>;
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
        Ok(format!("memory://{}", key))
    }

    async fn is_healthy(&self) -> Result<crate::health::HealthStatus> {
        Ok(crate::health::HealthStatus::healthy("blob", "memory", 0))
    }
}

use std::path::{Path, PathBuf};

/// Local filesystem implementation of BlobStore.
#[derive(Clone)]
pub struct LocalBlobStore {
    base_path: PathBuf,
}

impl LocalBlobStore {
    pub fn new<P: AsRef<Path>>(base_path: P) -> Self {
        Self {
            base_path: base_path.as_ref().to_path_buf(),
        }
    }

    fn resolve_path(&self, key: &str) -> Result<PathBuf> {
        // Simple security check against directory traversal
        let clean_key = key.trim_start_matches('/').replace('\\', "/");
        if clean_key.contains("..") {
            anyhow::bail!("Invalid blob key: directory traversal detected");
        }
        Ok(self.base_path.join(clean_key))
    }

    fn meta_path(&self, blob_path: &Path) -> PathBuf {
        let mut path_str = blob_path.as_os_str().to_os_string();
        path_str.push(".meta.json");
        PathBuf::from(path_str)
    }
}

#[async_trait]
impl BlobStore for LocalBlobStore {
    async fn put(&self, key: &str, data: &[u8], metadata: Option<Metadata>) -> Result<()> {
        let blob_path = self.resolve_path(key)?;
        if let Some(parent) = blob_path.parent() {
            tokio::fs::create_dir_all(parent).await?;
        }

        tokio::fs::write(&blob_path, data).await?;

        if let Some(meta) = metadata {
            let meta_path = self.meta_path(&blob_path);
            let meta_json = serde_json::to_vec(&meta)?;
            tokio::fs::write(meta_path, meta_json).await?;
        }

        Ok(())
    }

    async fn get(&self, key: &str) -> Result<Option<(Vec<u8>, Metadata)>> {
        let blob_path = self.resolve_path(key)?;
        if !blob_path.exists() {
            return Ok(None);
        }

        let data = tokio::fs::read(&blob_path).await?;
        let meta_path = self.meta_path(&blob_path);
        let metadata = if meta_path.exists() {
            let meta_bytes = tokio::fs::read(meta_path).await?;
            serde_json::from_slice(&meta_bytes).unwrap_or_default()
        } else {
            Metadata::default()
        };

        Ok(Some((data, metadata)))
    }

    async fn delete(&self, key: &str) -> Result<()> {
        let blob_path = self.resolve_path(key)?;
        if blob_path.exists() {
            let _ = tokio::fs::remove_file(&blob_path).await;
        }
        let meta_path = self.meta_path(&blob_path);
        if meta_path.exists() {
            let _ = tokio::fs::remove_file(meta_path).await;
        }
        Ok(())
    }

    async fn list(&self, prefix: Option<&str>) -> Result<Vec<String>> {
        if !self.base_path.exists() {
            return Ok(Vec::new());
        }

        let mut results = Vec::new();
        let mut dirs_to_visit = vec![self.base_path.clone()];

        while let Some(dir) = dirs_to_visit.pop() {
            let mut entries = tokio::fs::read_dir(dir).await?;
            while let Some(entry) = entries.next_entry().await? {
                let path = entry.path();
                if path.is_dir() {
                    dirs_to_visit.push(path);
                } else {
                    let file_name = path.file_name().and_then(|n| n.to_str()).unwrap_or("");
                    if file_name.ends_with(".meta.json") {
                        continue;
                    }
                    if let Ok(rel_path) = path.strip_prefix(&self.base_path) {
                        let rel_str = rel_path.to_string_lossy().replace('\\', "/");
                        if let Some(pref) = prefix {
                            if rel_str.starts_with(pref) {
                                results.push(rel_str);
                            }
                        } else {
                            results.push(rel_str);
                        }
                    }
                }
            }
        }

        results.sort();
        Ok(results)
    }

    async fn presign_url(&self, key: &str, _expiry: Duration) -> Result<String> {
        let blob_path = self.resolve_path(key)?;
        let abs_path = std::fs::canonicalize(&blob_path).unwrap_or(blob_path);
        Ok(format!("file://{}", abs_path.to_string_lossy()))
    }

    async fn is_healthy(&self) -> Result<crate::health::HealthStatus> {
        let start = std::time::Instant::now();
        let health_file = self.base_path.join(".health_check_probe");
        match tokio::fs::create_dir_all(&self.base_path).await {
            Ok(_) => match tokio::fs::write(&health_file, b"ok").await {
                Ok(_) => {
                    let _ = tokio::fs::remove_file(&health_file).await;
                    let latency = start.elapsed().as_millis() as u64;
                    Ok(crate::health::HealthStatus::healthy("blob", "local", latency))
                }
                Err(e) => {
                    let latency = start.elapsed().as_millis() as u64;
                    Ok(crate::health::HealthStatus::unhealthy("blob", "local", &format!("cannot write to blob dir: {}", e), latency))
                }
            },
            Err(e) => {
                let latency = start.elapsed().as_millis() as u64;
                Ok(crate::health::HealthStatus::unhealthy("blob", "local", &format!("cannot create blob dir: {}", e), latency))
            }
        }
    }
}

#[cfg(feature = "s3-backend")]
pub struct S3BlobStore {
    client: aws_sdk_s3::Client,
    bucket: String,
}

#[cfg(feature = "s3-backend")]
impl S3BlobStore {
    pub async fn new(config: &crate::config::BlobConfig) -> Result<Self> {
        let bucket = config.bucket.clone().unwrap_or_else(|| "goy-store".to_string());
        let region = config.region.clone().unwrap_or_else(|| "us-east-1".to_string());

        let mut config_loader = aws_config::defaults(aws_config::BehaviorVersion::latest())
            .region(aws_sdk_s3::config::Region::new(region));

        if let (Some(ak), Some(sk)) = (&config.access_key, &config.secret_key) {
            let creds = aws_credential_types::Credentials::new(
                ak.clone(),
                sk.clone(),
                None,
                None,
                "goy-store",
            );
            config_loader = config_loader.credentials_provider(creds);
        }

        let sdk_config = config_loader.load().await;

        let mut s3_config_builder = aws_sdk_s3::config::Builder::from(&sdk_config);

        if let Some(endpoint) = &config.endpoint {
            s3_config_builder = s3_config_builder.endpoint_url(endpoint);
        }

        if config.force_path_style.unwrap_or(true) {
            s3_config_builder = s3_config_builder.force_path_style(true);
        }

        let client = aws_sdk_s3::Client::from_conf(s3_config_builder.build());
        Ok(Self { client, bucket })
    }
}

#[cfg(feature = "s3-backend")]
#[async_trait]
impl BlobStore for S3BlobStore {
    async fn put(&self, key: &str, data: &[u8], metadata: Option<Metadata>) -> Result<()> {
        let mut req = self
            .client
            .put_object()
            .bucket(&self.bucket)
            .key(key)
            .body(aws_sdk_s3::primitives::ByteStream::from(data.to_vec()));

        if let Some(meta) = metadata {
            if let Some(ct) = meta.content_type {
                req = req.content_type(ct);
            }
            for (k, v) in meta.custom {
                req = req.metadata(k, v);
            }
        }

        req.send().await?;
        Ok(())
    }

    async fn get(&self, key: &str) -> Result<Option<(Vec<u8>, Metadata)>> {
        let res = self
            .client
            .get_object()
            .bucket(&self.bucket)
            .key(key)
            .send()
            .await;

        match res {
            Ok(output) => {
                let content_type = output.content_type().map(|s| s.to_string());
                let custom = output
                    .metadata()
                    .map(|m| m.iter().map(|(k, v)| (k.clone(), v.clone())).collect())
                    .unwrap_or_default();

                let bytes = output.body.collect().await?.into_bytes().to_vec();
                Ok(Some((
                    bytes,
                    Metadata {
                        content_type,
                        custom,
                    },
                )))
            }
            Err(aws_sdk_s3::error::SdkError::ServiceError(err))
                if err.err().is_no_such_key() =>
            {
                Ok(None)
            }
            Err(e) => Err(anyhow::anyhow!("S3 GetObject failed: {}", e)),
        }
    }

    async fn delete(&self, key: &str) -> Result<()> {
        self.client
            .delete_object()
            .bucket(&self.bucket)
            .key(key)
            .send()
            .await?;
        Ok(())
    }

    async fn list(&self, prefix: Option<&str>) -> Result<Vec<String>> {
        let mut results = Vec::new();
        let mut continuation_token = None;

        loop {
            let mut req = self.client.list_objects_v2().bucket(&self.bucket);
            if let Some(p) = prefix {
                req = req.prefix(p);
            }
            if let Some(token) = continuation_token {
                req = req.continuation_token(token);
            }

            let output = req.send().await?;
            if let Some(contents) = output.contents {
                for obj in contents {
                    if let Some(key) = obj.key {
                        results.push(key);
                    }
                }
            }

            if output.is_truncated.unwrap_or(false) {
                continuation_token = output.next_continuation_token;
            } else {
                break;
            }
        }

        results.sort();
        Ok(results)
    }

    async fn presign_url(&self, key: &str, expiry: Duration) -> Result<String> {
        let presigning_config = aws_sdk_s3::presigning::PresigningConfig::expires_in(expiry)?;
        let presigned = self
            .client
            .get_object()
            .bucket(&self.bucket)
            .key(key)
            .presigned(presigning_config)
            .await?;
        Ok(presigned.uri().to_string())
    }

    async fn is_healthy(&self) -> Result<crate::health::HealthStatus> {
        let start = std::time::Instant::now();
        let timeout_fut = tokio::time::timeout(
            Duration::from_secs(3),
            self.client.head_bucket().bucket(&self.bucket).send(),
        );

        match timeout_fut.await {
            Ok(Ok(_)) => {
                let latency = start.elapsed().as_millis() as u64;
                Ok(crate::health::HealthStatus::healthy("blob", "s3", latency))
            }
            Ok(Err(e)) => {
                let latency = start.elapsed().as_millis() as u64;
                Ok(crate::health::HealthStatus::unhealthy("blob", "s3", &e.to_string(), latency))
            }
            Err(_) => {
                let latency = start.elapsed().as_millis() as u64;
                Ok(crate::health::HealthStatus::unhealthy("blob", "s3", "health check timed out (3s)", latency))
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_local_blob_store() {
        let temp_dir = tempfile::tempdir().unwrap();
        let store = LocalBlobStore::new(temp_dir.path());

        let mut custom = HashMap::new();
        custom.insert("author".to_string(), "goy".to_string());
        let meta = Metadata {
            content_type: Some("text/plain".to_string()),
            custom,
        };

        // Put
        store.put("certs/node.crt", b"CERT_DATA", Some(meta)).await.unwrap();

        // Get
        let (data, retrieved_meta) = store.get("certs/node.crt").await.unwrap().unwrap();
        assert_eq!(data, b"CERT_DATA");
        assert_eq!(retrieved_meta.content_type.as_deref(), Some("text/plain"));
        assert_eq!(retrieved_meta.custom.get("author").map(|s| s.as_str()), Some("goy"));

        // List
        let list = store.list(Some("certs/")).await.unwrap();
        assert_eq!(list, vec!["certs/node.crt".to_string()]);

        // Delete
        store.delete("certs/node.crt").await.unwrap();
        assert!(store.get("certs/node.crt").await.unwrap().is_none());
    }
}