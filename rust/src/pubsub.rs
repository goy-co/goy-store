//! Pub/Sub Store Contract
//!
//! Publish and subscribe to events between service instances. Used for key revocation,
//! cache invalidation, state synchronization, configuration notifications.

use anyhow::Result;
use async_trait::async_trait;
use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::{broadcast, RwLock};

#[derive(Clone)]
pub struct Message {
    pub channel: String,
    pub payload: Vec<u8>,
    pub timestamp: i64,
}

#[async_trait]
pub trait PubSubStore: Send + Sync {
    async fn publish(&self, channel: &str, message: &[u8]) -> Result<()>;
    async fn subscribe(&self, channels: &[&str]) -> Result<tokio_stream::wrappers::BroadcastStream<Message>>;
    async fn unsubscribe(&self, channels: &[&str]) -> Result<()>;
}

/// In-memory implementation of PubSubStore for testing and local development.
pub struct MemoryPubSubStore {
    channels: Arc<RwLock<HashMap<String, broadcast::Sender<Message>>>>,
}

impl Default for MemoryPubSubStore {
    fn default() -> Self {
        Self {
            channels: Arc::new(RwLock::new(HashMap::new())),
        }
    }
}

#[async_trait]
impl PubSubStore for MemoryPubSubStore {
    async fn publish(&self, channel: &str, message: &[u8]) -> Result<()> {
        let channels = self.channels.read().await;
        if let Some(tx) = channels.get(channel) {
            let msg = Message {
                channel: channel.to_string(),
                payload: message.to_vec(),
                timestamp: chrono::Utc::now().timestamp(),
            };
            let _ = tx.send(msg); // Ignore send errors if no receivers
        }
        Ok(())
    }

    async fn subscribe(&self, channels: &[&str]) -> Result<tokio_stream::wrappers::BroadcastStream<Message>> {
        // For simplicity, this implementation only supports subscribing to a single channel at a time
        // A real implementation would merge multiple streams or use a different architecture
        let channel = channels.first().copied().unwrap_or("default");
        
        let mut chans = self.channels.write().await;
        let tx = chans.entry(channel.to_string()).or_insert_with(|| {
            broadcast::channel(1000).0
        });
        
        let rx = tx.subscribe();
        Ok(tokio_stream::wrappers::BroadcastStream::new(rx))
    }

    async fn unsubscribe(&self, _channels: &[&str]) -> Result<()> {
        // In a broadcast channel, unsubscribing is handled by dropping the receiver.
        // We might clean up empty channels here in a more sophisticated implementation.
        Ok(())
    }
}

#[cfg(feature = "redis-backend")]
pub struct RedisPubSubStore {
    conn: redis::aio::ConnectionManager,
    client: redis::Client,
    channels: Arc<RwLock<HashMap<String, broadcast::Sender<Message>>>>,
}

#[cfg(feature = "redis-backend")]
impl RedisPubSubStore {
    pub async fn new(url: &str) -> Result<Self> {
        let client = redis::Client::open(url)?;
        let conn = redis::aio::ConnectionManager::new(client.clone()).await?;
        Ok(Self {
            conn,
            client,
            channels: Arc::new(RwLock::new(HashMap::new())),
        })
    }

    pub fn from_connection_manager(conn: redis::aio::ConnectionManager, client: redis::Client) -> Self {
        Self {
            conn,
            client,
            channels: Arc::new(RwLock::new(HashMap::new())),
        }
    }
}

#[cfg(feature = "redis-backend")]
#[async_trait]
impl PubSubStore for RedisPubSubStore {
    async fn publish(&self, channel: &str, message: &[u8]) -> Result<()> {
        let mut conn = self.conn.clone();
        let () = redis::AsyncCommands::publish(&mut conn, channel, message).await?;
        Ok(())
    }

    async fn subscribe(&self, channels: &[&str]) -> Result<tokio_stream::wrappers::BroadcastStream<Message>> {
        use tokio_stream::StreamExt;

        let channel_name = channels.first().copied().unwrap_or("default");
        let (tx, rx) = {
            let mut chans = self.channels.write().await;
            if let Some(existing_tx) = chans.get(channel_name) {
                (existing_tx.clone(), existing_tx.subscribe())
            } else {
                let (tx, rx) = broadcast::channel(1000);
                chans.insert(channel_name.to_string(), tx.clone());

                let client = self.client.clone();
                let ch_str = channel_name.to_string();
                let tx_clone = tx.clone();

                tokio::spawn(async move {
                    if let Ok(mut pubsub) = client.get_async_pubsub().await {
                        if pubsub.subscribe(&ch_str).await.is_ok() {
                            let mut msg_stream = pubsub.on_message();
                            while let Some(msg) = msg_stream.next().await {
                                let payload: Vec<u8> = msg.get_payload().unwrap_or_default();
                                let m = Message {
                                    channel: ch_str.clone(),
                                    payload,
                                    timestamp: chrono::Utc::now().timestamp(),
                                };
                                if tx_clone.send(m).is_err() {
                                    // No active listeners left
                                    break;
                                }
                            }
                        }
                    }
                });

                (tx, rx)
            }
        };

        let _ = tx;
        Ok(tokio_stream::wrappers::BroadcastStream::new(rx))
    }

    async fn unsubscribe(&self, channels: &[&str]) -> Result<()> {
        let mut chans = self.channels.write().await;
        for &ch in channels {
            chans.remove(ch);
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_memory_pubsub_store() {
        use tokio_stream::StreamExt;

        let store = MemoryPubSubStore::default();
        let mut stream = store.subscribe(&["events"]).await.unwrap();

        store.publish("events", b"test-payload").await.unwrap();

        if let Some(Ok(msg)) = stream.next().await {
            assert_eq!(msg.channel, "events");
            assert_eq!(msg.payload, b"test-payload");
        } else {
            panic!("expected message from stream");
        }
    }
}