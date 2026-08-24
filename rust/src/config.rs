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
    #[serde(default)]
    pub resilience: ResilienceConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ResilienceConfig {
    #[serde(default = "default_max_retries")]
    pub max_retries: u32,
    #[serde(default = "default_base_backoff_ms")]
    pub base_backoff_ms: u64,
    #[serde(default = "default_circuit_breaker_threshold")]
    pub circuit_breaker_threshold: u32,
    #[serde(default = "default_circuit_breaker_reset_seconds")]
    pub circuit_breaker_reset_seconds: u64,
    #[serde(default = "default_operation_timeout_seconds")]
    pub operation_timeout_seconds: u64,
}

fn default_max_retries() -> u32 {
    3
}
fn default_base_backoff_ms() -> u64 {
    100
}
fn default_circuit_breaker_threshold() -> u32 {
    5
}
fn default_circuit_breaker_reset_seconds() -> u64 {
    30
}
fn default_operation_timeout_seconds() -> u64 {
    5
}

impl Default for ResilienceConfig {
    fn default() -> Self {
        Self {
            max_retries: default_max_retries(),
            base_backoff_ms: default_base_backoff_ms(),
            circuit_breaker_threshold: default_circuit_breaker_threshold(),
            circuit_breaker_reset_seconds: default_circuit_breaker_reset_seconds(),
            operation_timeout_seconds: default_operation_timeout_seconds(),
        }
    }
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
    pub access_key: Option<String>,
    pub secret_key: Option<String>,
    pub force_path_style: Option<bool>,
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
