# ADR-002: Valkey as the Redis-Compatible Backend

**Status:** Accepted
**Date:** 2026-08-27

## Context

The current KV, SortedSet, and PubSub backends use Redis as the production implementation. Redis has moved to the SSPL (Server Side Public License), which is not OSI-approved and creates licensing ambiguity for a project released under the Goy Source Available License (GSAL).

Valkey is the open-source continuation of Redis under the Linux Foundation. It is wire-compatible with Redis — same RESP protocol, same commands, same client libraries. The `redis` Rust crate and `go-redis` Go library work with Valkey without code changes.

## Decision

Replace Redis with Valkey as the default production backend for KV, SortedSet, and PubSub contracts.

- **Config:** `backend = "valkey"` is the canonical string. `"redis"` is accepted as an alias for backward compatibility.
- **Docker:** `valkey/valkey:7.2-alpine` replaces `redis:7-alpine` in test infrastructure.
- **Code:** No implementation changes. The `redis` crate and `go-redis` library connect to Valkey identically.

## Rationale

Valkey provides the same functionality as Redis with a BSD license, which is compatible with the Goy Source Available License's intent. The SSPL's restrictions on offering the service as a managed offering create legal ambiguity that is unnecessary to accept when a wire-compatible, Apache-2.0-licensed alternative exists.

The code is identical because Valkey is a fork. The only changes are:
1. Docker image reference
2. Default config string
3. Documentation

## Consequences

- **Positive:** License clarity. Valkey's BSD license is unambiguous.
- **Positive:** Zero code changes to store implementations.
- **Positive:** Existing Redis configurations continue to work via the `"redis"` alias.
- **Positive:** Same performance characteristics, same operational model.
- **Negative:** Valkey 7.2 is newer than Redis 7-alpine; minor command differences may exist in edge cases. Mitigated by E2E tests against the actual Valkey container.
- **Negative:** Some managed Redis services (AWS ElastiCache, Google Cloud Memorystore) do not yet offer Valkey. Mitigated by supporting both config strings — consumers can use `"redis"` for managed Redis and `"valkey"` for self-hosted.

## Alternatives Considered

**Option A — Stay with Redis:** Rejected due to SSPL licensing concerns. The legal ambiguity is not worth the operational familiarity.

**Option B — Support both equally without a default:** Rejected because it creates decision paralysis for consumers. Valkey is the recommended default; Redis is the compatible fallback.
