//go:build e2e

package e2e

import (
	"context"
	"os"
	"testing"

	goystore "github.com/goy-co/goy-store/go"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

func getRedisURL() string {
	if u := os.Getenv("REDIS_URL"); u != "" {
		return u
	}
	return "redis://127.0.0.1:6379"
}

func getPostgresURL() string {
	if u := os.Getenv("DATABASE_URL"); u != "" {
		return u
	}
	return "postgres://test:test@127.0.0.1:5432/goy_store_test?sslmode=disable"
}

func createTestStore(t *testing.T) goystore.GoyStore {
	tempDir, err := os.MkdirTemp("", "goy-store-blob-e2e-*")
	if err != nil {
		t.Fatalf("failed to create temp dir for blob: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})

	cfg := &goystore.Config{
		KV: goystore.KVConfig{
			Backend: "redis",
			URL:     getRedisURL(),
		},
		Relational: goystore.RelationalConfig{
			Backend: "postgres",
			URL:     getPostgresURL(),
		},
		SortedSet: goystore.SortedSetConfig{
			Backend: "redis",
			URL:     getRedisURL(),
		},
		PubSub: goystore.PubSubConfig{
			Backend: "redis",
			URL:     getRedisURL(),
		},
		Blob: goystore.BlobConfig{
			Backend: "local",
			Path:    tempDir,
		},
		Resilience: goystore.ResilienceConfig{
			MaxRetries:                 3,
			BaseBackoffMS:              10,
			CircuitBreakerThreshold:    5,
			CircuitBreakerResetSeconds: 1,
			OperationTimeoutSeconds:    2,
		},
	}

	store, err := goystore.NewStore(cfg)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	return store
}

func flushRedis(t *testing.T) {
	opt, err := redis.ParseURL(getRedisURL())
	if err != nil {
		t.Fatalf("failed to parse redis url: %v", err)
	}
	client := redis.NewClient(opt)
	defer client.Close()

	if err := client.FlushAll(context.Background()).Err(); err != nil {
		t.Fatalf("failed to flush redis: %v", err)
	}
}

func resetPostgresTables(t *testing.T) {
	conn, err := pgx.Connect(context.Background(), getPostgresURL())
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	defer conn.Close(context.Background())

	_, _ = conn.Exec(context.Background(), "DROP TABLE IF EXISTS schema_migrations CASCADE;")
	_, _ = conn.Exec(context.Background(), "DROP TABLE IF EXISTS users CASCADE;")
	_, _ = conn.Exec(context.Background(), "DROP TABLE IF EXISTS nodes CASCADE;")
}

func getMinIOEndpoint() string {
	if u := os.Getenv("MINIO_ENDPOINT"); u != "" {
		return u
	}
	return "http://127.0.0.1:9000"
}

func getMinIOAccessKey() string {
	if k := os.Getenv("MINIO_ACCESS_KEY"); k != "" {
		return k
	}
	return "minioadmin"
}

func getMinIOSecretKey() string {
	if k := os.Getenv("MINIO_SECRET_KEY"); k != "" {
		return k
	}
	return "minioadmin"
}

func getMinIOBucket() string {
	if b := os.Getenv("MINIO_BUCKET"); b != "" {
		return b
	}
	return "goy-store-test"
}

func setupMinIOBucket(t *testing.T) *goystore.S3BlobStore {
	cfg := goystore.BlobConfig{
		Backend:        "s3",
		Endpoint:       getMinIOEndpoint(),
		Region:         "us-east-1",
		Bucket:         getMinIOBucket(),
		AccessKey:      getMinIOAccessKey(),
		SecretKey:      getMinIOSecretKey(),
		ForcePathStyle: true,
	}

	store, err := goystore.NewS3BlobStore(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to create s3 blob store: %v", err)
	}

	return store
}
