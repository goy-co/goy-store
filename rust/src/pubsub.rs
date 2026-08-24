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