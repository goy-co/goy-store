# Goy Store — Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for the Goy Store project.

An ADR captures a significant technical choice: the decision, the options considered, the trade-offs, and the rationale. It is written at the time of the decision, not after.

## Index

| ADR | Title | Status | Date |
|---|---|---|---|
| [001](001-contracts-as-native-traits.md) | Contracts as Native Traits, Not gRPC Services | Accepted | 2026-08-27 |
| [002](002-valkey-as-redis-compatible-backend.md) | Valkey as the Redis-Compatible Backend | Accepted | 2026-08-27 |
| [003](003-tidb-as-relational-backend.md) | TiDB as RelationalStore Backend (MySQL Wire Protocol) | Accepted | 2026-08-27 |
| [004](004-nats-as-pubsub-backend.md) | NATS as PubSubStore Backend (Core NATS, Not JetStream) | Accepted | 2026-08-27 |
| [005](005-sqlite-as-embedded-dev-backend.md) | SQLite as Embedded Dev Backend for RelationalStore | Accepted | 2026-08-27 |
| [006](006-transaction-support-in-rust-relational-trait.md) | Transaction Support in Rust RelationalStore Trait | Accepted | 2026-08-27 |

## Superseded ADRs

None yet.

## How to Propose a New ADR

1. Create a new file: `adr/NNN-title-in-kebab-case.md`
2. Use the format: Title, Status, Date, Context, Decision, Rationale, Consequences, Alternatives Considered
3. Add an entry to the index table above
4. Reference the ADR in the relevant code and documentation
