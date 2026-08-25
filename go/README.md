# Goy Store (Go Module)

[![Go Reference](https://pkg.go.dev/badge/github.com/goy-co/goy-store/go.svg)](https://pkg.go.dev/github.com/goy-co/goy-store/go)
[![License](https://img.shields.io/badge/license-Goy%20Source%20Available%20License-blue.svg)](LICENSE)

**Unified Persistence Abstraction for the Goy Platform (Go)**.

`github.com/goy-co/goy-store/go` provides standardized, idiomatic Go interfaces for Key-Value, Relational, Sorted Set, Pub/Sub, and Blob storage.

---

## 📦 Features

- **`KVStore`**: In-Memory and Redis backend (`go-redis/v9`) with TTL and `SetIfNotExists`.
- **`RelationalStore`**: In-Memory and PostgreSQL backend (`pgx/v5`) with connection pooling and schema migrations.
- **`SortedSetStore`**: In-Memory and Redis backend with score range lookups and member expiration.
- **`PubSubStore`**: In-Memory and Redis Pub/Sub channels with channel-based streaming.
- **`BlobStore`**: In-Memory, Local Filesystem, and AWS S3 / MinIO / Cloudflare R2 backend with metadata and presigned URLs.
- **Observability**: Built-in Prometheus metrics per contract and backend.
- **Resilience**: Configurable exponential retry with jitter and circuit breaker protection.
- **Health Checks**: Context-aware, non-blocking health check probes for each backend.

---

## 🚀 Installation

```bash
go get github.com/goy-co/goy-store/go@v0.1.1-alpha
```

---

## 💡 Quickstart

```go
package main

import (
	"context"
	"fmt"
	"time"

	goystore "github.com/goy-co/goy-store/go"
)

func main() {
	ctx := context.Background()

	// Load from TOML or create default configuration
	cfg := goystore.DefaultConfig()
	store, err := goystore.NewStore(cfg)
	if err != nil {
		panic(err)
	}

	// 1. Key-Value Store
	ttl := 5 * time.Minute
	if err := store.KV().Set(ctx, "session:abc", []byte("user_data"), &ttl); err != nil {
		panic(err)
	}

	val, exists, err := store.KV().Get(ctx, "session:abc")
	if err != nil || !exists {
		panic("key not found")
	}
	fmt.Printf("Fetched: %s\n", string(val))

	// 2. Blob Store (Presigned URLs)
	url, err := store.Blob().PresignURL(ctx, "certs/node.crt", 10*time.Minute)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Download URL: %s\n", url)

	// 3. Health Check
	health := store.HealthCheck(ctx)
	fmt.Printf("Health status: %s\n", health.State)
}
```

---

## 📄 License

Goy Source Available License (GSAL). See [LICENSE](LICENSE) for terms.
