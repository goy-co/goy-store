//! Key-Value Store Contract
//!
//! Basic key-value operations with optional TTL. Used for sessions, caches, tokens, temporary state.

use anyhow::Result;
use async_trait::async_trait;
use std::collections::HashMap;
use std::sync::Arc;
use std::time::Duration;
use tokio::sync::RwLock;

#[async_trait]
pub trait KvStore: Send + Sync {
    async fn get(&self, key: &str) -> Result<Option<Vec<u8>>>;
    async fn set(&self, key: &str, value: &[u8], ttl: Option<Duration>) -> Result<()>;
    async fn delete(&self, key: &str) -> Result<()>;
    async fn exists(&self, key: &str) -> Result<bool>;
    async fn set_if_not_exists(
        &self,
        key: &str,
        value: &[u8],
        ttl: Option<Duration>,
    ) -> Result<bool>;
    async fn is_healthy(&self) -> Result<crate::health::HealthStatus>;
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

    async fn set_if_not_exists(
        &self,
        key: &str,
        value: &[u8],
        _ttl: Option<Duration>,
    ) -> Result<bool> {
        let mut data = self.data.write().await;
        if data.contains_key(key) {
            Ok(false)
        } else {
            data.insert(key.to_string(), value.to_vec());
            Ok(true)
        }
    }

    async fn is_healthy(&self) -> Result<crate::health::HealthStatus> {
        Ok(crate::health::HealthStatus::healthy("kv", "memory", 0))
    }
}

#[cfg(feature = "redis-backend")]
pub struct RedisKvStore {
    conn: redis::aio::ConnectionManager,
}

#[cfg(feature = "redis-backend")]
impl RedisKvStore {
    pub async fn new(url: &str) -> Result<Self> {
        let client = redis::Client::open(url)?;
        let conn = redis::aio::ConnectionManager::new(client).await?;
        Ok(Self { conn })
    }

    pub fn from_connection_manager(conn: redis::aio::ConnectionManager) -> Self {
        Self { conn }
    }

    pub fn connection_manager(&self) -> redis::aio::ConnectionManager {
        self.conn.clone()
    }
}

#[cfg(feature = "redis-backend")]
#[async_trait]
impl KvStore for RedisKvStore {
    async fn get(&self, key: &str) -> Result<Option<Vec<u8>>> {
        let mut conn = self.conn.clone();
        let result: Option<Vec<u8>> = redis::AsyncCommands::get(&mut conn, key).await?;
        Ok(result)
    }

    async fn set(&self, key: &str, value: &[u8], ttl: Option<Duration>) -> Result<()> {
        let mut conn = self.conn.clone();
        if let Some(ttl) = ttl {
            let seconds = ttl.as_secs();
            if seconds > 0 {
                let () = redis::AsyncCommands::set_ex(&mut conn, key, value, seconds).await?;
            } else {
                let millis = ttl.as_millis() as u64;
                let () = redis::cmd("PSETEX")
                    .arg(key)
                    .arg(millis)
                    .arg(value)
                    .query_async(&mut conn)
                    .await?;
            }
        } else {
            let () = redis::AsyncCommands::set(&mut conn, key, value).await?;
        }
        Ok(())
    }

    async fn delete(&self, key: &str) -> Result<()> {
        let mut conn = self.conn.clone();
        let () = redis::AsyncCommands::del(&mut conn, key).await?;
        Ok(())
    }

    async fn exists(&self, key: &str) -> Result<bool> {
        let mut conn = self.conn.clone();
        let exists: bool = redis::AsyncCommands::exists(&mut conn, key).await?;
        Ok(exists)
    }

    async fn set_if_not_exists(
        &self,
        key: &str,
        value: &[u8],
        ttl: Option<Duration>,
    ) -> Result<bool> {
        let mut conn = self.conn.clone();
        if let Some(ttl) = ttl {
            let seconds = ttl.as_secs();
            let mut cmd = redis::cmd("SET");
            cmd.arg(key).arg(value).arg("NX");
            if seconds > 0 {
                cmd.arg("EX").arg(seconds);
            } else {
                cmd.arg("PX").arg(ttl.as_millis() as u64);
            }
            let res: Option<String> = cmd.query_async(&mut conn).await?;
            Ok(res.is_some())
        } else {
            let set: bool = redis::AsyncCommands::set_nx(&mut conn, key, value).await?;
            Ok(set)
        }
    }

    async fn is_healthy(&self) -> Result<crate::health::HealthStatus> {
        let start = std::time::Instant::now();
        let mut conn = self.conn.clone();
        let ping_fut = async move {
            let cmd = redis::cmd("PING");
            cmd.query_async::<_, String>(&mut conn).await
        };
        let timeout_fut = tokio::time::timeout(std::time::Duration::from_secs(2), ping_fut);

        match timeout_fut.await {
            Ok(Ok(_)) => {
                let latency = start.elapsed().as_millis() as u64;
                Ok(crate::health::HealthStatus::healthy("kv", "redis", latency))
            }
            Ok(Err(e)) => {
                let latency = start.elapsed().as_millis() as u64;
                Ok(crate::health::HealthStatus::unhealthy(
                    "kv",
                    "redis",
                    &e.to_string(),
                    latency,
                ))
            }
            Err(_) => {
                let latency = start.elapsed().as_millis() as u64;
                Ok(crate::health::HealthStatus::unhealthy(
                    "kv",
                    "redis",
                    "health check timed out (2s)",
                    latency,
                ))
            }
        }
    }
}
