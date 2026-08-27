# ADR-005: SQLite as Embedded Dev Backend for RelationalStore

**Status:** Accepted
**Date:** 2026-08-27

## Context

The README lists SQLite as a dev backend for RelationalStore. It is not implemented. This is a gap between documentation and implementation.

SQLite is an embedded, zero-dependency relational database. It is ideal for:
- Local development without Docker
- Single-node deployments (edge devices, small instances)
- Unit tests that need a real database without external infrastructure

## Decision

Add SQLite as a dev/test backend for RelationalStore.

- **Rust:** `sqlx` with `sqlite` feature (already in `Cargo.toml`). New `SqliteRelationalStore` wrapping `sqlx::SqlitePool`.
- **Go:** `modernc.org/sqlite` (pure Go, no CGO). New `sqliteRelational` implementation.
- **Config:** `backend = "sqlite"`, `url = "sqlite://path/to/db.db"` or `sqlite::memory:` for in-memory.

## Rationale

SQLite fills the gap between in-memory (no persistence) and PostgreSQL/TiDB (external service). It enables:
- **Local development:** No Docker required. A single binary with a local file.
- **Edge deployments:** Single-node Goy instances can use SQLite instead of requiring an external database.
- **Testing:** Faster than spinning up a PostgreSQL container for unit tests that need real SQL.

`modernc.org/sqlite` is chosen for Go because it is pure Go (no CGO), which simplifies cross-compilation and avoids build complexity. `sqlx` already supports SQLite in Rust.

## Consequences

- **Positive:** Closes the documentation-implementation gap.
- **Positive:** Enables zero-dependency local development.
- **Positive:** `sqlx` provides the same API for SQLite as for PostgreSQL/MySQL, minimizing implementation effort.
- **Positive:** Pure Go SQLite driver avoids CGO complexity.
- **Negative:** SQLite does not support all PostgreSQL/MySQL features (e.g., `ALTER TABLE DROP COLUMN` has limitations, no stored procedures). Consumers must write portable SQL.
- **Negative:** SQLite is single-writer. Not suitable for high-write-concurrency workloads. This is acceptable for a dev/test backend.
- **Negative:** Go requires a new dependency (`modernc.org/sqlite`).

## Alternatives Considered

**Option A — `mattn/go-sqlite3` (CGO):** Rejected. CGO complicates cross-compilation and requires a C toolchain. `modernc.org/sqlite` is pure Go with no external dependencies.

**Option B — Only in-memory for dev, no SQLite:** Rejected. In-memory does not persist across restarts. SQLite provides persistence without external infrastructure, which is valuable for edge deployments and local development.

**Option C — DuckDB instead of SQLite:** Rejected. DuckDB is optimized for analytical workloads (OLAP), not transactional workloads (OLTP). The `RelationalStore` contract is OLTP-oriented (CRUD, transactions, migrations). SQLite is the correct fit.
