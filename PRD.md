# PRD: goy-store v1.0 — The Last Persistence Abstraction We'll Ever Need

| Field | Value |
|---|---|
| **Document Version** | 1.1-draft |
| **Date** | 2026-08-27 |
| **Author** | Northstar (CPO) |
| **Status** | Draft — Pending Review |
| **Stakeholders** | Atlas (CEO), Forge (CTO), Craft (Engineering), Sentinel (QA), Tekt (DevOps) |

---

## 1. Problem Statement

### The Problem

The Goy platform is growing. Today we have three infrastructure binaries — `goy-node`, `goy-relay`, and `goy-vpn` — each managing state differently. `goy-node` uses SQLite for edge data. `goy-relay` uses TiDB and Valkey for control-plane data. `goy-vpn` uses TiDB and Valkey for the WireGuard plane.

Each service currently implements its own persistence logic, its own retry semantics, its own health checks, its own metrics. This means:

- **Duplicated effort**: every new service reinvents the wheel.
- **Inconsistent behavior**: retry logic in one service differs from another.
- **Operational blind spots**: no unified observability across persistence layers.
- **Onboarding friction**: a new engineer must learn N different persistence implementations.

`goy-store` v0.1.x proved the abstraction works. But it was released as a standalone foundation — no service has migrated to it yet. It lacks the hardening, the enterprise features, and the operational rigor required for production workloads.

Additionally, the current implementation has gaps the ADRs have already identified:
- The Rust `RelationalStore` trait lacks transaction support — a contract mismatch with Go.
- The `proto/` directory implies a gRPC architecture that doesn't exist and never will.
- NATS is the chosen PubSub backend but isn't implemented.
- SQLite is documented but not implemented.

### The Opportunity

`goy-store` v1.0 becomes the single, unified persistence abstraction for the entire Goy platform. Every infrastructure binary consumes storage through one API. One retry policy. One set of metrics. One health check. One security model.

This is the last persistence abstraction we'll ever need.

---

## 2. Vision & Success Metrics

### Vision

`goy-store` v1.0 is the production-grade, enterprise-hardened persistence layer that every Goy infrastructure binary uses. It is boring, reliable, and fast. Developers don't think about it — they just use it.

### Success Metrics

| Metric | Target | Measurement |
|---|---|---|
| **Adoption** | 100% of new Goy infrastructure binaries use goy-store | Code audit — zero direct driver usage |
| **Latency (KV/SortedSet)** | p99 < 1ms for local backends (Valkey, In-Memory) | Prometheus histogram |
| **Latency (Relational)** | p99 < 10ms for TiDB/SQLite | Prometheus histogram |
| **Availability** | 99.9% uptime | Health check endpoint + external probe |
| **Mean Time to Recovery (MTTR)** | < 5 minutes from detection to mitigation | Incident post-mortem data |
| **Zero data loss** | No unrecoverable data loss in production | Audit log + backup verification |
| **Developer onboarding** | New engineer productive with goy-store in < 1 hour | Self-reported survey |
| **Contract parity** | Rust and Go expose identical capabilities | API diff audit |

---

## 3. Scope

### In Scope

| Area | Details |
|---|---|
| **Core Contracts** | KVStore, RelationalStore, SortedSetStore, PubSubStore, BlobStore |
| **Production Backends** | Valkey (Redis-compatible), TiDB (MySQL wire protocol), S3/MinIO/R2, NATS (Core), SQLite (embedded), In-Memory |
| **Multi-tenancy** | Tenant-aware operations with isolation guarantees |
| **Security** | Encryption at rest + in transit, structured audit logging |
| **Resilience** | Retry with jitter, circuit breaker, graceful degradation |
| **Observability** | Prometheus metrics, structured tracing, health probes |
| **Disaster Recovery** | Point-in-time recovery, backup/restore |
| **Testing** | Unit, integration, chaos, and load testing |
| **Documentation** | OpenAPI/gRPC spec, Rust docs, Go docs, migration guide |
| **Migration** | Clean-slate migration from v0.1.x (breaking changes allowed) |
| **Rust-Go contract parity** | Rust `RelationalStore` gains transaction support to match Go |
| **Proto cleanup** | Remove `proto/` directory — contracts are native traits only |

### Out of Scope

| Area | Rationale |
|---|---|
| CockroachDB / PostgreSQL native drivers | Extras — scheduled after v1.0 |
| JetStream as PubSub mode | Core NATS only; JetStream requires a new contract (`StreamingStore`) |
| Distributed transactions (2PC) across contracts | Explicitly out of scope |
| Sidecar / remote service deployment | Embedded library only for v1.0 |
| New contracts beyond the 5 existing | Harden what exists; no new capabilities |
| Caching layer / read-through cache | Not required for v1.0 |
| Event sourcing / CDC | Not required for v1.0 |

---

## 4. Consumer Profiles

### 4.1 goy-node (Edge)

| Attribute | Value |
|---|---|
| **Primary Backend** | SQLite (embedded) |
| **Use Case** | Edge data persistence — local state, configuration, cached metadata |
| **Consistency** | Strong (SQLite guarantees this) |
| **Latency Sensitivity** | Low — edge workloads are not latency-critical |
| **Tenancy** | Single-tenant per node instance |
| **Deployment** | Embedded in binary, runs on edge devices |

### 4.2 goy-relay (Control Plane)

| Attribute | Value |
|---|---|
| **Primary Backends** | TiDB (relational), Valkey (KV/SortedSet/PubSub) |
| **Use Case** | Control-plane data — routing tables, node registrations, session state, heartbeats |
| **Consistency** | Strong — control-plane correctness depends on it |
| **Latency Sensitivity** | High — sub-millisecond p99 for KV/SortedSet |
| **Tenancy** | Multi-tenant — serves multiple customers from shared infrastructure |
| **Deployment** | Embedded in binary, runs on cloud VMs |

### 4.3 goy-vpn (WireGuard Plane)

| Attribute | Value |
|---|---|
| **Primary Backends** | TiDB (relational), Valkey (KV/SortedSet/PubSub) |
| **Use Case** | WireGuard plane — peer registrations, key distribution, connection state |
| **Consistency** | Strong — key distribution must be correct |
| **Latency Sensitivity** | High — sub-millisecond p99 for KV/SortedSet |
| **Tenancy** | Multi-tenant — serves multiple VPN customers |
| **Deployment** | Embedded in binary, runs on cloud VMs |

---

## 5. Functional Requirements

### 5.1 KVStore

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| KV-01 | Get a value by key | Returns `Some(value)` if exists, `None` if not. Operation completes in < 1ms p99 (Valkey/In-Memory). |
| KV-02 | Set a value with optional TTL | Stores value. TTL of `None` means no expiration. Returns error if backend is unreachable after retries. |
| KV-03 | Delete a value by key | Removes key. Returns error if key doesn't exist (idempotent — no error on missing key). |
| KV-04 | Check existence | Returns `true` if key exists, `false` otherwise. |
| KV-05 | Set if not exists (atomic) | Sets value only if key doesn't exist. Returns `true` if set, `false` if key already existed. Atomic at the backend level. |
| KV-06 | Increment (atomic) | Atomically increments a numeric value. Returns new value. |
| KV-07 | Batch get | Returns values for multiple keys in a single round-trip where the backend supports it. |
| KV-08 | Batch set | Sets multiple key-value pairs in a single round-trip where the backend supports it. |

### 5.2 RelationalStore

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| REL-01 | Execute a query (read) | Returns rows. Supports parameterized queries to prevent SQL injection. |
| REL-02 | Execute a statement (write) | Returns affected row count. Supports parameterized statements. |
| REL-03 | Transaction support (Rust + Go) | Begin, commit, rollback. Transactions are ACID at the backend level. **Rust trait must add `transaction` method to match Go.** Closure-based API: auto-commit on success, auto-rollback on error. |
| REL-04 | Schema migrations | Apply versioned migrations. Migrations are idempotent and reversible where possible. Migration table uses `TIMESTAMP` (not `TIMESTAMPTZ`) for cross-backend compatibility. |
| REL-05 | Connection pooling | Configurable pool size. Pool exhaustion returns a clear error, not a hang. |
| REL-06 | Prepared statements | Support for prepared statements where the backend supports it. |
| REL-07 | Batch execution | Execute multiple statements in a single transaction. |

### 5.3 SortedSetStore

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| SS-01 | Add member with score | Adds member to sorted set. Updates score if member exists. |
| SS-02 | Remove member | Removes member from sorted set. No error if member doesn't exist. |
| SS-03 | Range by score | Returns members with scores in [min, max]. Supports limit/offset. |
| SS-04 | Range by rank | Returns members by rank (index). Supports limit/offset. |
| SS-05 | Count members in score range | Returns count of members with scores in [min, max]. |
| SS-06 | Remove range by score | Removes all members with scores in [min, max]. Returns count removed. |
| SS-07 | Get member score | Returns score of a member. `None` if member doesn't exist. |
| SS-08 | Get member rank | Returns rank (0-indexed) of a member. `None` if member doesn't exist. |
| SS-09 | Set expiration on key | Sorted set key expires after TTL. |

### 5.4 PubSubStore

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| PS-01 | Publish message to channel | Publishes to all subscribers. Returns error if backend is unreachable. |
| PS-02 | Subscribe to channel | Returns a stream of messages. Stream ends on unsubscribe or error. |
| PS-03 | Unsubscribe from channel | Stops the subscription. Resources are cleaned up. |
| PS-04 | Pattern subscribe | Subscribes to channels matching a pattern (where backend supports it). |
| PS-05 | Message ordering | Messages from the same publisher are received in order (per channel). |
| PS-06 | Core NATS backend | NATS implemented as a production backend using **core NATS** (fire-and-forget, at-most-once). No JetStream. |

### 5.5 BlobStore

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| BLB-01 | Put object | Stores object with metadata. Overwrites if key exists. |
| BLB-02 | Get object | Returns object bytes and metadata. Error if key doesn't exist. |
| BLB-03 | Delete object | Removes object. No error if key doesn't exist. |
| BLB-04 | List objects | Returns keys matching prefix. Supports pagination. |
| BLB-05 | Presign URL | Generates a time-limited URL for direct upload/download. |
| BLB-06 | Object metadata | Returns metadata (size, content-type, custom headers) without fetching object. |
| BLB-07 | Multipart upload | Supports large object upload in parts (where backend supports it). |

### 5.6 Multi-Tenancy

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| MT-01 | Tenant context propagation | Every operation accepts a tenant context. Tenant ID is propagated to the backend. |
| MT-02 | Tenant isolation | Operations for tenant A cannot access tenant B's data. Enforced at the application layer (key prefixing / row-level security). |
| MT-03 | Per-tenant configuration | Each tenant can have different backend configurations (e.g., different Valkey instances). |
| MT-04 | Tenant-aware metrics | All Prometheus metrics include a `tenant_id` label. |
| MT-05 | Tenant-aware audit logs | All audit log entries include the tenant ID. |

---

## 6. Non-Functional Requirements

### 6.1 Performance

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| NFR-01 | KVStore p99 latency | < 1ms for Valkey and In-Memory backends (measured at the goy-store API boundary, excluding network). |
| NFR-02 | SortedSetStore p99 latency | < 1ms for Valkey and In-Memory backends. |
| NFR-03 | RelationalStore p99 latency | < 10ms for TiDB and SQLite backends (simple queries). |
| NFR-04 | BlobStore p99 latency | < 50ms for S3/MinIO (PUT/GET of 1KB object). |
| NFR-05 | PubSubStore p99 latency | < 5ms for NATS and Valkey backends. |
| NFR-06 | Throughput | KVStore sustains 100K ops/sec per core on Valkey backend. |
| NFR-07 | Memory overhead | goy-store adds < 5MB memory overhead beyond the backend's own usage. |

### 6.2 Availability

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| NFR-08 | SLA | 99.9% uptime (8.76 hours downtime/year). |
| NFR-09 | Graceful degradation | When a backend is unreachable, goy-store returns a clear error within the operation timeout. No hangs. |
| NFR-10 | Circuit breaker | Opens after 5 consecutive failures. Half-opens after 30s. Closes after 1 successful probe. |
| NFR-11 | Retry with jitter | Retries up to 3 times with exponential backoff (100ms base, 2x exponent, ±25% jitter). |

### 6.3 Scalability

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| NFR-12 | Horizontal scaling | goy-store instances are stateless and can be replicated behind a load balancer. |
| NFR-13 | Connection pooling | RelationalStore supports configurable connection pools (default: 20 connections). |
| NFR-14 | Concurrent access | All contracts are safe for concurrent use from multiple goroutines/tasks. |

---

## 7. Security & Compliance

### 7.1 Encryption

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| SEC-01 | Encryption in transit | All backend connections use TLS by default. Configurable to disable for trusted networks (not recommended). |
| SEC-02 | Encryption at rest | BlobStore supports server-side encryption (SSE-S3, SSE-KMS). RelationalStore relies on backend encryption (TiDB/SQLite encryption extensions). |
| SEC-03 | Key management | Encryption keys are never hardcoded. Retrieved from environment variables or a secrets manager. |

### 7.2 Authentication & Authorization

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| SEC-04 | Backend authentication | All backend connections authenticate using credentials (passwords, tokens, IAM roles). |
| SEC-05 | Credential storage | Credentials are never logged or exposed in error messages. Loaded from environment variables or secrets files. |

### 7.3 Audit Logging

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| SEC-06 | Structured audit logs | Every mutating operation (Set, Delete, Put, Publish, Execute) emits a structured audit log entry. |
| SEC-07 | Audit log format | JSON format with fields: `timestamp`, `tenant_id`, `contract`, `operation`, `key`, `backend`, `outcome`, `duration_ms`. |
| SEC-08 | Audit log delivery | Audit logs are written to a configurable sink (stdout, file, or external system via hook). |
| SEC-09 | Non-repudiation | Audit log entries cannot be modified after writing (append-only sink). |

---

## 8. Observability

### 8.1 Metrics (Prometheus)

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| OBS-01 | Operation duration histograms | Per contract, per operation, per backend, per tenant. |
| OBS-02 | Error counters | Per contract, per operation, per backend, per error type. |
| OBS-03 | Retry counters | Per contract, per operation, per backend. |
| OBS-04 | Circuit breaker state gauge | Per contract, per backend. |
| OBS-05 | Health check status gauge | Per contract, per backend. |
| OBS-06 | Health check duration histogram | Per contract, per backend. |
| OBS-07 | Connection pool gauges | Per relational backend (active, idle, waiting). |

### 8.2 Health Checks

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| OBS-08 | Liveness probe | Returns `healthy` if the store can serve reads. |
| OBS-09 | Readiness probe | Returns `ready` if all backends are reachable. |
| OBS-10 | Per-backend health | Each backend has an independent health status. |
| OBS-11 | Health check timeout | Redis/Valkey: 2s, TiDB: 3s, S3/MinIO: 3s, NATS: 2s, Local: 1s. |

### 8.3 Tracing

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| OBS-12 | OpenTelemetry integration | Operations produce spans compatible with OpenTelemetry. |
| OBS-13 | Trace context propagation | Trace context is propagated to backends where supported. |

---

## 9. Resilience & Disaster Recovery

### 9.1 Retry & Circuit Breaker

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| RES-01 | Configurable retry policy | `max_retries`, `base_backoff_ms`, `max_backoff_ms` are configurable per store instance. |
| RES-02 | Exponential backoff with jitter | Delay = `base_backoff * 2^attempt ± 25% jitter`. |
| RES-03 | Circuit breaker per backend | Each backend has its own circuit breaker. |
| RES-04 | Fast failure when circuit is open | Operations fail immediately with `CircuitOpen` error when the circuit is open. |

### 9.2 Disaster Recovery

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| DR-01 | Point-in-time recovery | RelationalStore supports point-in-time recovery via TiDB's binlog / SQLite's WAL archiving. |
| DR-02 | Backup & restore | BlobStore objects are backed up to a secondary location. RelationalStore supports logical backups. |
| DR-03 | Backup verification | Backups are automatically verified (checksum / test restore) on a schedule. |
| DR-04 | Recovery time objective (RTO) | < 30 minutes to restore from backup. |
| DR-05 | Recovery point objective (RPO) | < 5 minutes of data loss in a disaster scenario. |

---

## 10. Backend Specifications

### 10.1 Valkey (Redis-Compatible)

| Attribute | Value |
|---|---|
| **Contracts** | KVStore, SortedSetStore, PubSubStore |
| **Config string** | `"valkey"` (canonical), `"redis"` (alias for backward compatibility) |
| **Rust driver** | `redis` crate (wire-compatible with Valkey) |
| **Go driver** | `go-redis/v9` (wire-compatible with Valkey) |
| **Docker** | `valkey/valkey:7.2-alpine` |
| **License rationale** | BSD license, compatible with GSAL. Avoids SSPL ambiguity. |

### 10.2 TiDB (MySQL Wire Protocol)

| Attribute | Value |
|---|---|
| **Contracts** | RelationalStore |
| **Config string** | `"tidb"` or `"mysql"` |
| **Rust driver** | `sqlx` with `mysql` feature |
| **Go driver** | `go-sql-driver/mysql` |
| **Docker** | TiDB standalone mode (embedded TiKV) for E2E tests |
| **Migration table** | Uses `TIMESTAMP` (not `TIMESTAMPTZ`) for cross-backend compatibility |

### 10.3 NATS (Core)

| Attribute | Value |
|---|---|
| **Contracts** | PubSubStore |
| **Config string** | `"nats"` |
| **Rust driver** | `async-nats` crate |
| **Go driver** | `nats.go` (official client) |
| **Docker** | `nats:2-alpine` |
| **Semantics** | Core NATS only — fire-and-forget, at-most-once. No JetStream. |

### 10.4 SQLite (Embedded)

| Attribute | Value |
|---|---|
| **Contracts** | RelationalStore |
| **Config string** | `"sqlite"` |
| **Rust driver** | `sqlx` with `sqlite` feature |
| **Go driver** | `modernc.org/sqlite` (pure Go, no CGO) |
| **URL format** | `sqlite://path/to/db.db` or `sqlite::memory:` |

### 10.5 S3/MinIO/R2

| Attribute | Value |
|---|---|
| **Contracts** | BlobStore |
| **Config string** | `"s3"` |
| **Rust driver** | `aws-sdk-s3` |
| **Go driver** | `aws-sdk-go-v2` |
| **Docker** | `minio/minio` for E2E tests |

### 10.6 In-Memory

| Attribute | Value |
|---|---|
| **Contracts** | All (dev/test only) |
| **Config string** | `"memory"` |

---

## 11. Testing Strategy

| Level | Scope | Tooling | Success Criteria |
|---|---|---|---|
| **Unit** | Individual contract logic, mock backends | Rust: `cargo test`, Go: `go test` | > 90% code coverage on contract implementations |
| **Integration** | Contracts against real backends (Docker) | Rust: `cargo test --test e2e`, Go: `go test -tags=e2e` | All contracts pass against all production backends |
| **Chaos** | Backend failures, network partitions, latency injection | Custom chaos scripts + Docker | Circuit breaker opens/retries fire correctly; no data corruption |
| **Load** | Sustained throughput, latency under load | Rust: `criterion`, Go: `testing/bench` | p99 latency targets met at 2x expected production load |

---

## 12. Documentation

| ID | Requirement | Acceptance Criteria |
|---|---|---|
| DOC-01 | Rust API docs | Published to docs.rs. Every public item has a doc comment with examples. |
| DOC-02 | Go API docs | Published to pkg.go.dev. Every exported symbol has a doc comment. |
| DOC-03 | Configuration reference | Complete TOML configuration reference with all options, defaults, and examples. Includes Valkey/Redis alias, TiDB/Mysql, NATS, SQLite config strings. |
| DOC-04 | Migration guide | Step-by-step guide for migrating from v0.1.x to v1.0 (breaking changes documented). |
| DOC-05 | Architecture overview | Updated README with the architecture diagram, backend matrix, and consumer profiles. |
| DOC-06 | Operational runbook | How to monitor, troubleshoot, and recover goy-store in production. |
| DOC-07 | Security guide | How to configure encryption, audit logging, and credential management. |

---

## 13. Migration & Cleanup from v0.1.x

Breaking changes are allowed. The migration path is:

1. **Audit current usage**: Identify all consumers of goy-store v0.1.x.
2. **API diff**: Publish a detailed API diff (v0.1.x → v1.0) with before/after code examples.
3. **Migration guide**: Step-by-step guide for each breaking change.
4. **Parallel run (optional)**: For critical services, support running v0.1.x and v1.0 side-by-side during migration (feature flag).
5. **Deprecation window**: v0.1.x is deprecated upon v1.0 release. Security patches only for 90 days.

### Explicit Breaking Changes in v1.0

| Change | Impact | Mitigation |
|---|---|---|
| **Remove `proto/` directory** | Any code referencing proto-generated types will break. | Proto was never compiled or served — no runtime impact. Update any documentation references. |
| **Rust `RelationalStore` gains `transaction` method** | External implementors of the trait must add the method. | goy-store is internal — all implementors are in-tree. |
| **Valkey as canonical config string** | Existing configs using `"redis"` continue to work via alias. | No migration needed — alias is automatic. |
| **Migration table uses `TIMESTAMP`** | PostgreSQL-specific `TIMESTAMPTZ` code must change. | `TIMESTAMP` works in PostgreSQL, MySQL/TiDB, SQLite. |

---

## 14. Acceptance Criteria (Definition of Done)

`goy-store` v1.0 is considered done when:

- [ ] All functional requirements (Section 5) pass integration tests against all production backends.
- [ ] All non-functional requirements (Section 6) pass load tests.
- [ ] All security requirements (Section 7) pass a security review.
- [ ] All observability requirements (Section 8) are verified in a staging environment.
- [ ] All resilience requirements (Section 9) pass chaos tests.
- [ ] Rust and Go contracts are feature-equivalent (transaction parity confirmed).
- [ ] `proto/` directory is removed.
- [ ] All documentation (Section 12) is published and reviewed.
- [ ] Migration guide (Section 13) is complete and tested with at least one consumer service.
- [ ] Sentinel (QA) signs off on release readiness.
- [ ] Tekt (DevOps) confirms deployability.

---

## 15. Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| TiDB performance doesn't meet sub-ms p99 for KV workloads | Medium | High | Use Valkey as primary KV backend; TiDB for relational only |
| Multi-tenancy key prefixing leaks data | Low | Critical | Integration tests with tenant isolation chaos; code review by Forge |
| Breaking changes delay consumer migration | Medium | Medium | Migration guide + parallel run support; early engagement with consumer teams |
| Encryption at rest adds unacceptable latency | Low | Medium | Benchmark in staging; make encryption configurable per backend |
| Audit logging volume overwhelms sinks | Medium | High | Configurable sampling; async log delivery; sink backpressure handling |
| NATS core semantics (at-most-once) surprise consumers expecting persistence | Medium | Medium | Clear documentation of delivery semantics; health check includes NATS connection status |

---

## 16. Out of Scope (Explicitly)

The following are **not** part of goy-store v1.0 and will be addressed in future releases:

- CockroachDB / PostgreSQL native drivers
- JetStream as PubSub mode (requires new `StreamingStore` contract)
- Distributed transactions (2PC) across contracts
- Sidecar / remote service deployment model
- Caching layer / read-through cache
- Event sourcing / CDC
- GraphQL or REST API layer on top of contracts
- Automatic schema generation / ORM features

---

## Appendix A: Backend Matrix

| Contract | Valkey | TiDB | NATS | S3/MinIO/R2 | SQLite | In-Memory |
|---|---|---|---|---|---|---|
| **KVStore** | ✅ Primary | — | — | — | — | ✅ Dev/test |
| **RelationalStore** | — | ✅ Primary | — | — | ✅ Edge | ✅ Dev/test |
| **SortedSetStore** | ✅ Primary | — | — | — | — | ✅ Dev/test |
| **PubSubStore** | ✅ Primary | — | ✅ Primary | — | — | ✅ Dev/test |
| **BlobStore** | — | — | — | ✅ Primary | — | ✅ Dev/test |

---

## Appendix B: Consumer-Backend Mapping

| Consumer | KVStore | RelationalStore | SortedSetStore | PubSubStore | BlobStore |
|---|---|---|---|---|---|
| **goy-node** | In-Memory | SQLite | In-Memory | In-Memory | — |
| **goy-relay** | Valkey | TiDB | Valkey | Valkey / NATS | S3 (artifacts) |
| **goy-vpn** | Valkey | TiDB | Valkey | Valkey / NATS | S3 (certs) |

---

## Appendix C: ADR Cross-Reference

| ADR | Incorporated Into | PRD Section |
|---|---|---|
| ADR-001: Contracts as native traits, not gRPC | Migration & Cleanup | §13 |
| ADR-002: Valkey as Redis-compatible backend | Backend Specs, Config | §10.1 |
| ADR-003: TiDB as RelationalStore backend | Backend Specs, RelationalStore | §10.2, §5.2 |
| ADR-004: NATS as PubSubStore backend | Backend Specs, PubSubStore | §10.3, §5.4 |
| ADR-005: SQLite as embedded dev backend | Backend Specs, RelationalStore | §10.4, §5.2 |
| ADR-006: Transaction support in Rust RelationalStore | RelationalStore, Migration | §5.2, §13 |
