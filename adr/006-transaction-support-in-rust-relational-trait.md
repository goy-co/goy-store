# ADR-006: Transaction Support in Rust RelationalStore Trait

**Status:** Accepted
**Date:** 2026-08-27

## Context

The Go `RelationalStore` interface includes a `Transaction` method:

```go
Transaction(ctx context.Context, fn func(Tx) error) error
```

The Rust `RelationalStore` trait does not expose transaction support. This is a contract version mismatch — the two language implementations do not expose the same capabilities.

Transactions are a core relational database feature. The `RelationalStore` contract is used for "node registries, credentials, persistent configuration, audit logs" — all domains that require atomic multi-statement operations.

## Decision

Add transaction support to the Rust `RelationalStore` trait:

```rust
async fn transaction<F, Fut, R>(&self, f: F) -> Result<R>
where
    F: FnOnce(Transaction) -> Fut + Send,
    Fut: Future<Output = Result<R>> + Send,
    R: Send;
```

The `Transaction` struct provides the same interface as `RelationalStore` for the duration of the transaction:

```rust
pub struct Transaction<'a> {
    // implementation-specific handle
}

impl<'a> Transaction<'a> {
    pub async fn query(&self, sql: &str, params: &[Param]) -> Result<Rows>;
    pub async fn execute(&self, sql: &str, params: &[Param]) -> Result<u64>;
}
```

## Rationale

Both language implementations must expose the same capabilities. Go has transactions; Rust must too. The closure-based API matches the Go style and allows the transaction to be automatically committed on success or rolled back on error.

`sqlx` provides transactions for all supported backends:
- `sqlx::PgPool::begin()` → `sqlx::PgConnection` (PostgreSQL)
- `sqlx::MySqlPool::begin()` → `sqlx::MySqlConnection` (TiDB/MySQL)
- `sqlx::SqlitePool::begin()` → `sqlx::SqliteConnection` (SQLite)

The `MemoryRelationalStore` implements transactions as a no-op (the closure executes without a real transaction).

## Consequences

- **Positive:** Rust and Go contracts are now feature-equivalent.
- **Positive:** Consumers can perform atomic multi-statement operations in Rust.
- **Positive:** `sqlx` provides a consistent transaction API across all backends.
- **Negative:** The trait gains a new method with generic closures and futures, which increases API complexity. This is unavoidable — transactions are inherently more complex than single-statement operations.
- **Negative:** Existing implementors of `RelationalStore` (if any external) must add the `transaction` method. This is a breaking change to the trait. Mitigated by the fact that `goy-store` is an internal library and all implementors are in-tree.

## Alternatives Considered

**Option A — Separate `TransactionalRelationalStore` trait:** Rejected. This would fragment the contract and force consumers to check whether their backend supports transactions. Transactions are a core feature of relational databases; all production backends support them.

**Option B — `begin_transaction` returning a `Transaction` handle (explicit commit/rollback):** Rejected. The closure-based API is safer — it ensures the transaction is always committed or rolled back, even on panic. An explicit handle risks resource leaks if the consumer forgets to commit or roll back.

**Option C — No transactions in Rust:** Rejected. This would leave Rust consumers unable to perform atomic operations that Go consumers can perform. Unacceptable for a unified contract.
