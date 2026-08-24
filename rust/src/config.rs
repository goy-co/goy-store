//! Configuration for Goy Store
//!
//! Backend selection is made via TOML configuration, not code.

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct StoreConfig {
    pub kv: KvConfig,
    pub relational: RelationalConfig,
    pub sorted_set: SortedSetConfig,
    pub pubsub: PubSubConfig,
    pub blob: BlobConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct KvConfig {
    pub backend: String,
    pub url: Option<String>,
    pub pool_size: Option<usize>,
    pub timeout_ms: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct RelationalConfig {
    pub backend: String,
    pub url: Option<String>,
    pub pool_size: Option<usize>,
    pub max_retries: Option<u32>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct SortedSetConfig {
    pub backend: String,
    pub url: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct PubSubConfig {
    pub backend: String,
    pub url: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct BlobConfig {
    pub backend: String,
    pub endpoint: Option<String>,
    pub bucket: Option<String>,
    pub region: Option<String>,
    pub path: Option<String>, // For filesystem backend
}

impl StoreConfig {
    /// Loads configuration from a TOML file.
    pub fn from_file(path: &str) -> anyhow::Result<Self> {
        let content = std::fs::read_to_string(path)?;
        let config: Self = toml::from_str(&content)?;
        Ok(config)
    }
}