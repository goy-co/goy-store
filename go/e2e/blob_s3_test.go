//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	goystore "github.com/goy-co/goy-store/go"
)

func TestS3BlobStore_OperationsAgainstMinIO(t *testing.T) {
	store := setupMinIOBucket(t)
	ctx := context.Background()

	key := "releases/v1.0.0/app-binary.tar.gz"
	payload := []byte{0xCA, 0xFE, 0xBA, 0xBE, 0x01, 0x02, 0x03, 0x04}
	meta := &goystore.Metadata{
		ContentType: "application/gzip",
		Custom: map[string]string{
			"version": "1.0.0",
			"commit":  "abcdef",
		},
	}

	// 1. Put
	if err := store.Put(ctx, key, payload, meta); err != nil {
		t.Fatalf("Put to S3 failed: %v", err)
	}

	// 2. Get
	fetched, fetchedMeta, exists, err := store.Get(ctx, key)
	if err != nil || !exists {
		t.Fatalf("Get from S3 failed: exists=%v, err=%v", exists, err)
	}
	if !bytes.Equal(fetched, payload) {
		t.Fatalf("data mismatch from S3")
	}
	if fetchedMeta.ContentType != "application/gzip" || fetchedMeta.Custom["version"] != "1.0.0" {
		t.Fatalf("metadata mismatch: %+v", fetchedMeta)
	}

	// 3. List
	prefix := "releases"
	list, err := store.List(ctx, &prefix)
	if err != nil || len(list) != 1 || list[0] != key {
		t.Fatalf("List failed: list=%+v, err=%v", list, err)
	}

	// 4. PresignURL
	url, err := store.PresignURL(ctx, key, 5*time.Minute)
	if err != nil || !strings.Contains(url, "http://") || !strings.Contains(url, key) {
		t.Fatalf("PresignURL failed: url=%s, err=%v", url, err)
	}

	// 5. Health Check
	health, err := store.IsHealthy(ctx)
	if err != nil || health.State != goystore.HealthHealthy || health.Backend != "s3" {
		t.Fatalf("IsHealthy failed: health=%+v, err=%v", health, err)
	}

	// 6. Delete
	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, _, exists, err = store.Get(ctx, key)
	if err != nil || exists {
		t.Fatalf("expected key to be deleted from S3, exists=%v, err=%v", exists, err)
	}
}
