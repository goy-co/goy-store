# ADR-004: NATS as PubSubStore Backend (Core NATS, Not JetStream)

**Status:** Accepted
**Date:** 2026-08-27

## Context

The README promises NATS as a production PubSubStore backend. NATS offers two modes:
- **Core NATS:** Fire-and-forget pub/sub. No persistence, no delivery guarantees beyond at-most-once.
- **JetStream:** Persistent streams with at-least-once delivery, consumer groups, and replay.

The `PubSubStore` contract defines `Publish`, `Subscribe`, and `Unsubscribe` — a fire-and-forget pub/sub model. It does not define consumer groups, message replay, or delivery guarantees.

## Decision

Add NATS as a production PubSubStore backend using **core NATS**, not JetStream.

- **Rust:** `async-nats` crate (already declared as optional dependency). New `NatsPubSubStore`.
- **Go:** `nats.go` (official client). New `natsPubSub` implementation.
- **Config:** `backend = "nats"`, `url = "nats://..."`.
- **Docker:** `nats:2-alpine` for E2E tests.

## Rationale

Core NATS matches the `PubSubStore` contract exactly:
- `Publish` → `nc.publish(subject, payload)`
- `Subscribe` → `nc.subscribe(subject)` → async message stream
- `Unsubscribe` → `sub.unsubscribe()`

JetStream is a different contract. It provides persistence, consumer groups, and delivery guarantees — features that the `PubSubStore` does not expose. Adding JetStream would either:
1. Add unused complexity (if we use JetStream but only expose core NATS semantics)
2. Require a new contract (e.g., `StreamingStore`) to expose JetStream features properly

Core NATS is simpler, faster, and matches the existing in-memory and Redis PubSub semantics.

## Consequences

- **Positive:** Core NATS matches the contract exactly — no semantic mismatch.
- **Positive:** Lower latency than JetStream (no persistence overhead).
- **Positive:** `async-nats` is already declared as an optional dependency in `Cargo.toml`.
- **Negative:** No message persistence. If a subscriber is disconnected, messages published during disconnection are lost. This is consistent with the contract's at-most-once semantics.
- **Negative:** Go requires a new dependency (`nats.go`).
- **Negative:** If a future milestone requires persistent streams, a new contract (`StreamingStore`) will be needed rather than extending `PubSubStore`.

## Alternatives Considered

**Option A — JetStream:** Rejected. JetStream provides features the contract does not expose. Using JetStream behind a core NATS interface adds complexity without benefit. A future `StreamingStore` contract can expose JetStream features properly.

**Option B — Both core NATS and JetStream behind the same contract:** Rejected. This would create a contract with two semantic modes, making it impossible for consumers to reason about delivery guarantees. The contract must have a single, clear semantic.
