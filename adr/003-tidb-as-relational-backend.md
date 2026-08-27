# ADR-003: TiDB as RelationalStore Backend (MySQL Wire Protocol)

**Status:** Accepted
**Date:** 2026-08-27

## Context

The README promises CockroachDB as a production RelationalStore backend. CockroachDB speaks the PostgreSQL wire protocol, which would allow reuse of the existing `sqlx` PostgreSQL driver (`PgPool`).

TiDB is an alternative distributed SQL database that speaks the **MySQL wire protocol**. It provides horizontal scalability with MySQL compatibility. Switching from CockroachDB to TiDB changes the driver requirements.

## Decision

Add TiDB as a production RelationalStore backend using the MySQL wire protocol.

- **Rust:** `sqlx` with `mysql` feature. New `TiDbRelationalStore` wrapping `sqlx::MySqlPool`.
- **Go:** `go-sql-driver/mysql`. New `tidbRelational` implementation.
- **Config:** `backend = "tidb"` or `backend = "mysql"`.
- **Migration table:** Change `TIMESTAMPTZ` to `TIMESTAMP` for cross-backend compatibility (works in PostgreSQL, MySQL, TiDB, SQLite).

## Rationale

TiDB provides horizontal scalability with broader MySQL ecosystem compatibility. The `RelationalStore` contract is raw SQL, so consumers write backend-compatible SQL. TiDB's distributed SQL layer handles sharding transparently.

The migration table currently uses `TIMESTAMPTZ`, which is PostgreSQL-specific. For cross-backend compatibility, we use `TIMESTAMP`:
- PostgreSQL: `TIMESTAMP` works (stores without timezone, which is sufficient for migration tracking)
- MySQL/TiDB: `TIMESTAMP` is the native type
- SQLite: `TEXT` or `DATETIME` — `TIMESTAMP` is accepted as a type alias

## Consequences

- **Positive:** TiDB provides horizontal scalability with MySQL compatibility.
- **Positive:** `sqlx` supports MySQL with the same API shape as PostgreSQL (`query`, `execute`, `begin`, etc.).
- **Positive:** Migration table becomes cross-backend compatible.
- **Negative:** Go requires a new dependency (`go-sql-driver/mysql`).
- **Negative:** TiDB's MySQL dialect has differences from PostgreSQL (e.g., `LIMIT` syntax, `INSERT ... ON DUPLICATE KEY UPDATE` vs. `ON CONFLICT`). Consumers writing raw SQL must be aware of backend-specific syntax.
- **Negative:** TiDB Docker image is larger than PostgreSQL. E2E tests will use TiDB standalone mode (embedded TiKV) to minimize resource usage.

## Alternatives Considered

**Option A — CockroachDB (PostgreSQL wire):** Originally planned. Rejected in favor of TiDB per product direction. CockroachDB would have reused the existing PostgreSQL driver, but TiDB's MySQL compatibility aligns better with the broader ecosystem.

**Option B — Native TiDB client instead of MySQL driver:** Rejected. TiDB is wire-compatible with MySQL; a native client would add a dependency for no functional gain. The MySQL driver works correctly with TiDB.

**Option C — Abstract SQL dialect behind the contract:** Rejected. The `RelationalStore` contract is intentionally raw SQL — it does not abstract the dialect. Consumers choose their backend and write compatible SQL. Adding a SQL abstraction layer would be a different product.
