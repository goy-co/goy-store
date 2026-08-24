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

func TestLocalBlobStore_Operations(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	key := "configs/node-1/app.json"
	payload := bytes.Repeat([]byte{0xAB}, 64*1024) // 64KB
	meta := &goystore.Metadata{
		ContentType: "application/json",
		Custom: map[string]string{
			"author": "goy-team",
		},
	}

	// 1. Put
	if err := store.Blob().Put(ctx, key, payload, meta); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// 2. Get
	fetched, fetchedMeta, exists, err := store.Blob().Get(ctx, key)
	if err != nil || !exists {
		t.Fatalf("Get failed: exists=%v, err=%v", exists, err)
	}
	if !bytes.Equal(fetched, payload) {
		t.Fatalf("fetched data does not match payload")
	}
	if fetchedMeta.ContentType != "application/json" || fetchedMeta.Custom["author"] != "goy-team" {
		t.Fatalf("metadata mismatch: %+v", fetchedMeta)
	}

	// 3. List
	prefix := "configs"
	list, err := store.Blob().List(ctx, &prefix)
	if err != nil || len(list) != 1 || list[0] != key {
		t.Fatalf("List failed: list=%+v, err=%v", list, err)
	}

	// 4. PresignURL
	url, err := store.Blob().PresignURL(ctx, key, time.Minute)
	if err != nil || !strings.HasPrefix(url, "file://") {
		t.Fatalf("PresignURL failed: url=%s, err=%v", url, err)
	}

	// 5. Delete
	if err := store.Blob().Delete(ctx, key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, _, exists, _ = store.Blob().Get(ctx, key)
	if exists {
		t.Fatalf("blob should not exist after deletion")
	}
}
