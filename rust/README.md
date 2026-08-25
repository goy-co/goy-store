# Goy Store (Rust Crate)

[![Crates.io](https://img.shields.io/crates/v/goy-store.svg)](https://crates.io/crates/goy-store)
[![Documentation](https://docs.rs/goy-store/badge.svg)](https://docs.rs/goy-store)
[![License](https://img.shields.io/badge/license-Goy%20Source%20Available%20License-blue.svg)](LICENSE)

**Unified Persistence Abstraction for the Goy Platform (Rust)**.

`goy-store` provides standardized asynchronous persistence contracts for Rust applications across Key-Value, Relational, Sorted Set, Pub/Sub, and Blob storage.

---

## 📦 Features

- **`KVStore`**: In-Memory and Redis backend with TTL and `set_if_not_exists`.
- **`RelationalStore`**: In-Memory and PostgreSQL backend with transaction support and schema migrations.
- **`SortedSetStore`**: In-Memory and Redis backend with score range queries and expiration.
- **`PubSubStore`**: In-Memory and Redis Pub/Sub channels with asynchronous streams.
- **`BlobStore`**: In-Memory, Local Filesystem, and AWS S3 / MinIO / Cloudflare R2 backend with metadata and presigned URLs.
- **Observability**: Built-in Prometheus metrics per contract and backend.
- **Resilience**: Configurable exponential retry with jitter and circuit breaker pattern.
- **Health Checks**: Asynchronous, non-blocking health check probes for each backend.

---

## 🚀 Installation

Add `goy-store` to your `Cargo.toml`:

```toml
[dependencies]
goy-store = { version = "0.1.1-alpha", features = ["all-backends"] }
```

### Feature Flags

- `memory` *(default)*: In-memory backends.
- `redis-backend`: Redis backend for KV, SortedSet, and PubSub.
- `sqlx-backend`: PostgreSQL / SQLite backend for Relational store.
- `s3-backend`: AWS S3 / MinIO / R2 backend for Blob store.
- `all-backends`: Enables all production backends.

---

## 💡 Quickstart

```rust
use goy_store::{config::StoreConfig, GoyStore};
use std::time::Duration;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let config = StoreConfig::from_file("config.toml")?;
    let store = GoyStore::from_config(&config).await?;

    // 1. Key-Value Store
    store.kv.set("session:abc", b"user_data", Some(Duration::from_secs(300))).await?;
    if let Some(val) = store.kv.get("session:abc").await? {
        println!("Fetched: {}", String::from_utf8_lossy(&val));
    }

    // 2. Sorted Set (Heartbeats / Timeline)
    store.sorted_set.add("active_nodes", "node-1", 1700000000.0).await?;
    let active = store.sorted_set.range_by_score("active_nodes", 1699999000.0, 1700000100.0, None).await?;
    println!("Active nodes: {:?}", active);

    // 3. Health Check
    let health = store.health_check().await;
    println!("Overall Status: {:?}", health.state);

    Ok(())
}
```

---

## 📄 License

Goy Source Available License (GSAL). See [LICENSE](LICENSE) for terms.
