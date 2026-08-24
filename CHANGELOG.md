# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0-alpha] - 2026-08-24

### Added
- **Core Persistence Contracts**:
  - `KVStore`: In-memory and Redis backends with TTL and conditional `SetIfNotExists`.
  - `RelationalStore`: In-memory and PostgreSQL backends with migrations (`schema_migrations`), raw queries, and DML execution.
  - `SortedSetStore`: In-memory and Redis backends supporting score range queries, score lookups, count, and range deletions.
  - `PubSubStore`: In-memory (broadcast channels) and Redis Pub/Sub backends.
  - `BlobStore`: In-memory, Local Filesystem, and S3/MinIO/R2 backends with metadata, custom headers, and presigned URLs.
- **Observability**:
  - Full Prometheus metrics export for operation durations, error counters, pool connections, and retry/circuit breaker status.
- **Resilience**:
  - Exponential backoff retry with jitter ($\pm 25\%$).
  - Circuit Breaker pattern with state machine (Closed $\to$ Open $\to$ Half-Open $\to$ Closed).
  - Per-contract operation timeouts.
- **Health Checks**:
  - Non-blocking `is_healthy` / `IsHealthy` checks on all backends.
  - Aggregated and concurrent `GoyStore::health_check()` consolidated reporting.
- **Testing & Tooling**:
  - Standalone Docker Compose infrastructure (Redis 7, PostgreSQL 16, MinIO).
  - E2E integration test suites for Rust (`cargo test --test e2e`) and Go (`go test -tags=e2e`).
  - Git hooks for Conventional Commits and formatting.
  - GitHub Actions CI/CD workflow.

### Notes
- `goy-store` v0.1.0-alpha is an independent library and persistence contract layer. Consumer service integrations (e.g. `goy-node`, `goy-relay`) will be established in subsequent milestone releases.
