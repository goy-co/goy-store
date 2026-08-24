# Goy Store

**Unified Persistence Abstraction** for the Goy Platform.

Goy Store is not a database. It is an internal interface (Rust crate + Go package) that defines standardized contracts for storage operations, allowing platform services to consume different backends — Redis, PostgreSQL, CockroachDB, SQLite, NATS, S3-compatible object storage — through the same API, without knowing backend-specific details.

## Core Contracts

Goy Store defines five fundamental contracts, each mappable to multiple backends:

1. **KV Store**: Ephemeral key-value operations with optional TTL (sessions, caches, tokens).
2. **Relational Store**: Transactional CRUD with ACID guarantees (registries, credentials, audit logs).
3. **Sorted Set Store**: Temporal range queries (heartbeats, stale node detection, priority queues).
4. **Pub/Sub Store**: Event propagation between service instances (key revocation, cache invalidation).
5. **Blob Store**: Object storage for large binary objects (certificates, backups, artifacts).

## Architecture

- **Protobuf**: Single source of truth for all contracts, ensuring API parity between Rust and Go.
- **Rust Crate**: `goy-store` for Rust consumers (e.g., Goy Node, Goy VPN).
- **Go Package**: `goy-store` for Go consumers (e.g., Goy Relay).
- **Backend Agnostic**: Swap backends via TOML configuration without changing application code.
- **Observability**: Built-in Prometheus metrics per contract, operation, and backend.
- **Resilience**: Configurable exponential backoff retry with jitter and circuit breaker protection.
- **Health Checks**: Standardized, non-blocking health checks and consolidated status reporting.

## Getting Started

### Rust

```toml
[dependencies]
goy-store = { path = "../goy-store/rust", features = ["all-backends"] }
```

### Go

```go
import "github.com/goy-co/goy-store/go"
```

## Supported Backends

| Contract | Production Backends | Local/Dev Backends |
|----------|---------------------|--------------------|
| KV | Redis, Valkey, Dragonfly | SQLite, In-Memory |
| Relational | PostgreSQL, CockroachDB, TiDB, Turso | SQLite, In-Memory |
| Sorted Set | Redis, PostgreSQL, SQLite | In-Memory |
| Pub/Sub | NATS, Redis Pub/Sub, PostgreSQL LISTEN | In-Memory |
| Blob | S3, R2, MinIO | Filesystem, In-Memory |

## Testing

### Unit Tests

```bash
make test
```

### Standalone E2E Integration Tests

The test suite runs against real Redis, PostgreSQL, and MinIO instances using Docker Compose:

```bash
# Run all E2E tests (starts infrastructure, runs Rust and Go tests, tears down)
make test-e2e

# Or start the test infrastructure manually:
make e2e-up

# Run individual test suites:
make test-e2e-rust
make test-e2e-go

# Tear down infrastructure:
make e2e-down
```

## Documentation

See the [Goy Platform Wiki](../wiki/Goy-Store-—-Unified-Persistence-Abstraction.md) for detailed specifications, contracts, and consumption patterns.