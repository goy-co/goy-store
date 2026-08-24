# Goy Store

[![CI](https://github.com/goy-co/goy-store/actions/workflows/ci.yml/badge.svg)](https://github.com/goy-co/goy-store/actions/workflows/ci.yml)
[![Release](https://img.shields.io/badge/release-v0.1.0--alpha-blue.svg)](https://github.com/goy-co/goy-store/releases)
[![License](https://img.shields.io/badge/license-Proprietary-red.svg)](LICENSE)

**Unified Persistence Abstraction** for the Goy Platform.

`goy-store` is not a database. It is an internal interface library (Rust crate + Go package) defining standardized persistence contracts. Platform services consume storage through a single API without hardcoding backend-specific drivers.

---

## 🏛️ Architecture Overview

```
┌────────────────────────────────────────────────────────┐
│               Consumer Services / APIs                 │
│             (goy-node, goy-relay, etc.)                │
└───────────────────────────┬────────────────────────────┘
                            │
┌───────────────────────────▼────────────────────────────┐
│                    GoyStore Facade                     │
│  (KVStore | RelationalStore | SortedSet | PubSub | Blob)│
└───────────────────────────┬────────────────────────────┘
                            │
┌───────────────────────────▼────────────────────────────┐
│               Resilience Layer (Retry & CB)            │
│       (Exponential Backoff + Jitter / Circuit Breaker) │
└───────────────────────────┬────────────────────────────┘
                            │
┌───────────────────────────▼────────────────────────────┐
│             Observability & Health Checks              │
│       (Prometheus Histograms/Gauges + Health Probes)   │
└───────────────────────────┬────────────────────────────┘
                            │
┌───────────────────────────▼────────────────────────────┐
│                   Backend Drivers                      │
│   (Redis | PostgreSQL | S3/MinIO | Local FS | Memory)  │
└────────────────────────────────────────────────────────┘
```

---

## 📦 Core Contracts

| Contract | Primary Purpose | Key Operations | Production Backends | Local / Dev Backends | Status |
|---|---|---|---|---|---|
| **`KVStore`** | Ephemeral key-value, caching, session state | `Get`, `Set` (TTL), `Delete`, `Exists`, `SetIfNotExists` | Redis, Valkey | In-Memory | Stable |
| **`RelationalStore`** | Transactional CRUD, registries, audit logs | `Query`, `Execute`, `Transaction`, `Migrate` | PostgreSQL, CockroachDB | SQLite, In-Memory | Stable |
| **`SortedSetStore`** | Score-ordered sets, temporal indexing, heartbeats | `Add`, `Remove`, `RangeByScore`, `Count`, `RemoveRange`, `Score` | Redis | In-Memory | Stable |
| **`PubSubStore`** | Event bus, node invalidation, lightweight messaging | `Publish`, `Subscribe`, `Unsubscribe` | Redis Pub/Sub, NATS | In-Memory | Stable |
| **`BlobStore`** | Object storage, binaries, certificates, artifacts | `Put`, `Get`, `Delete`, `List`, `PresignURL` | S3, MinIO, Cloudflare R2 | Filesystem, In-Memory | Stable |

---

## ⚙️ Configuration Reference

Configuration can be provided as a TOML file or built programmatically:

```toml
[kv]
backend = "redis"                       # "redis" or "memory"
url = "redis://127.0.0.1:6379"

[relational]
backend = "postgres"                   # "postgres" or "memory"
url = "postgres://test:test@127.0.0.1:5432/goy_store_test?sslmode=disable"
pool_size = 20

[sorted_set]
backend = "redis"                       # "redis" or "memory"
url = "redis://127.0.0.1:6379"

[pubsub]
backend = "redis"                       # "redis" or "memory"
url = "redis://127.0.0.1:6379"

[blob]
backend = "s3"                          # "s3", "local", or "memory"
endpoint = "http://127.0.0.1:9000"      # Optional: custom endpoint for MinIO / R2
bucket = "goy-store-blobs"
region = "us-east-1"
access_key = "minioadmin"               # Optional if using AWS environment variables
secret_key = "minioadmin"
force_path_style = true                 # Required for MinIO
path = "./data/blobs"                   # Used when backend is "local"

[resilience]
max_retries = 3                         # Maximum retry attempts
base_backoff_ms = 100                   # Initial backoff in milliseconds (2x exponent + 25% jitter)
circuit_breaker_threshold = 5           # Consecutively failed requests before opening circuit
circuit_breaker_reset_seconds = 30      # Timeout before half-open probe
operation_timeout_seconds = 5           # Per-operation timeout
```

### Environment Variable Overrides
- `REDIS_URL`: Overrides Redis connection URL across KV, SortedSet, and PubSub.
- `DATABASE_URL`: Overrides PostgreSQL connection string.
- `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_BUCKET`: Overrides S3/MinIO parameters.
- `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`: Standard AWS SDK credentials fallback.

---

## 🚀 Quickstart & Examples

### Rust Usage

```rust
use goy_store::{config::StoreConfig, GoyStore};
use std::time::Duration;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let config = StoreConfig::from_file("config.toml")?;
    let store = GoyStore::from_config(&config).await?;

    // KV Operation with TTL
    store.kv.set("session:abc", b"user_data", Some(Duration::from_secs(300))).await?;

    // Sorted Set (Heartbeat query)
    store.sorted_set.add("active_nodes", "node-1", 1700000000.0).await?;
    let active = store.sorted_set.range_by_score("active_nodes", 1699999000.0, 1700000100.0, None).await?;

    // Health Check
    let health = store.health_check().await;
    println!("Overall health: {:?}", health.state);

    Ok(())
}
```

Run executable example:
```bash
cd rust && cargo run --example basic_usage --all-features
```

### Go Usage

```go
package main

import (
	"context"
	"fmt"
	"time"

	goystore "github.com/goy-co/goy-store/go"
)

func main() {
	ctx := context.Background()
	cfg, err := goystore.LoadConfig("config.toml")
	if err != nil {
		panic(err)
	}

	store, err := goystore.NewStore(cfg)
	if err != nil {
		panic(err)
	}

	// KV Operation
	ttl := 5 * time.Minute
	_ = store.KV().Set(ctx, "session:abc", []byte("user_data"), &ttl)

	// Blob Presign URL
	url, _ := store.Blob().PresignURL(ctx, "certs/node.crt", 10*time.Minute)
	fmt.Printf("Download URL: %s\n", url)

	// Health Check
	health := store.HealthCheck(ctx)
	fmt.Printf("Health status: %s\n", health.State)
}
```

Run executable example:
```bash
cd go && go run ./examples/basic_usage
```

---

## 📊 Observability (Prometheus Metrics)

Every contract operation and background probe is automatically instrumented:

| Metric Name | Type | Labels | Description |
|---|---|---|---|
| `goy_store_kv_operation_duration_seconds` | Histogram | `operation`, `backend` | Latency of KV operations (`get`, `set`, `delete`, etc.) |
| `goy_store_relational_query_duration_seconds` | Histogram | `operation`, `backend` | Latency of relational queries (`query`, `execute`, `migrate`) |
| `goy_store_sorted_set_operation_duration_seconds` | Histogram | `operation`, `backend` | Latency of sorted set operations |
| `goy_store_pubsub_operation_duration_seconds` | Histogram | `operation`, `backend` | Latency of Pub/Sub publish and subscription setup |
| `goy_store_blob_operation_duration_seconds` | Histogram | `operation`, `backend` | Latency of object store operations (`get`, `put`, etc.) |
| `goy_store_errors_total` | Counter | `contract`, `operation`, `backend`, `error_type` | Total errors per contract and backend |
| `goy_store_retries_total` | Counter | `contract`, `operation`, `backend` | Number of executed retry attempts |
| `goy_store_circuit_breaker_state` | Gauge | `contract`, `backend` | Circuit breaker status (`0=closed`, `1=open`, `2=half-open`) |
| `goy_store_health_check_status` | Gauge | `contract`, `backend` | Health state (`1=healthy`, `0=unhealthy`, `2=degraded`) |
| `goy_store_health_check_duration_seconds` | Histogram | `contract`, `backend` | Latency of health check probes |

---

## 🛡️ Resilience & Health Checks

1. **Retry with Jitter**:
   Operations that experience transient network drops or driver errors are automatically retried up to `max_retries` with exponential backoff:
   \[
   \text{delay} = \text{base\_backoff} \times 2^{\text{attempt}} \pm 25\%\text{ jitter}
   \]
2. **Circuit Breaker**:
   If a backend encounters consecutive failures exceeding `circuit_breaker_threshold`, the circuit opens for `circuit_breaker_reset_seconds`, fast-failing traffic to prevent resource exhaustion before probing back with a half-open trial request.
3. **Health Probes**:
   - **Redis**: Asynchronous `PING` probe (2s timeout).
   - **PostgreSQL**: `SELECT 1` / `Ping` probe (3s timeout).
   - **S3 / MinIO**: `HeadBucket` probe (3s timeout).
   - **Local Blob**: Probe file read/write access.

---

## 🛠️ Local Development & Testing

All integration tests execute against real Docker services:

```bash
# Start Docker containers (Redis, Postgres, MinIO)
make e2e-up

# Run full test suite (Unit + E2E for Rust and Go)
make test-all

# Run individual suites
make test-e2e-rust
make test-e2e-go

# Stop and purge test containers
make e2e-down
```

### Git Hooks & Conventional Commits
To enable automatic formatting checks and Conventional Commit validation:
```bash
git config core.hooksPath .githooks
```

---

## 🚢 Release Process

Releases are fully automated via GitHub Actions [`.github/workflows/release.yml`](.github/workflows/release.yml):

1. Commit all changes using [Conventional Commits](CONTRIBUTING.md).
2. Create and push a semantic tag:
   ```bash
   git tag -a v0.1.0-alpha -m "release: goy-store v0.1.0-alpha"
   git push origin v0.1.0-alpha
   ```
3. The workflow automatically:
   - Validates all unit and E2E integration tests against real Docker containers.
   - Packages and verifies the Rust crate and Go module.
   - Generates release notes from commits via `git-cliff`.
   - Creates the GitHub Release with pre-release status handled automatically.

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines and git workflow details.

---

## ⚠️ Limitations of `v0.1.0-alpha`

- `goy-store` is released as a standalone foundation. Platform services (`goy-node`, `goy-relay`) will be migrated to consume `goy-store` in subsequent releases.
- NATS and CockroachDB driver wrappers are scheduled for `v0.2.0`.
- Distributed transaction coordination (2PC) across multiple contracts is not in scope.