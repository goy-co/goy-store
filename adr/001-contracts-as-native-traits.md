# ADR-001: Contracts as Native Traits, Not gRPC Services

**Status:** Accepted
**Date:** 2026-08-27

## Context

The repository contains `proto/store.proto`, which defines full gRPC service definitions for all five persistence contracts (KV, Relational, SortedSet, PubSub, Blob). However, the actual implementations use native language constructs — Rust traits and Go interfaces — and no gRPC server is generated or served from this codebase.

This creates a second source of truth. The proto file implies a distributed architecture where services communicate over the wire, but the actual architecture is a library consumed in-process. The proto file is dead weight: it is not compiled, not tested, and not served.

## Decision

Remove `proto/store.proto` and the entire `proto/` directory. Persistence contracts are defined exclusively as:
- **Rust:** `async_trait` traits in `rust/src/{kv,relational,sorted_set,pubsub,blob}.rs`
- **Go:** Interface types in `go/store.go`

The contract is the source of truth. It is versioned through the crate version (Rust) and module version (Go).

## Rationale

The Sovereignty Coefficient favors local compute over server dependency. gRPC moves logic to the wire — it introduces serialization overhead, network latency, deployment complexity, and a second definition that must be kept in sync with the implementation.

A trait/interface is the contract. It is:
- **Versioned** through semantic versioning of the crate/module
- **Testable** without network infrastructure
- **Consumed in-process** with zero serialization cost
- **The single source of truth** — no sync tax between proto and implementation

## Consequences

- **Positive:** Eliminates a misleading artifact. Reduces maintenance surface. Contracts are defined once.
- **Positive:** No protobuf compiler dependency in CI/CD.
- **Positive:** Faster builds (no code generation step).
- **Negative:** If a future milestone requires a standalone gRPC server, the proto definitions will need to be recreated. This is acceptable — the contract traits will serve as the authoritative reference.
- **Negative:** Any external documentation or service definitions that reference the proto file will need updating.

## Alternatives Considered

**Option A — Implement gRPC server:** Generate server stubs from proto, add a `goy-store-server` binary. Rejected because it adds a deployment surface, serialization overhead, and a second source of truth for no current benefit. The architecture is in-process, not service-oriented.

**Option B — Keep proto as documentation-only:** Rejected because an unmaintained proto file is worse than no proto file. It implies a contract that doesn't exist.
