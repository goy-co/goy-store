//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"
)

func TestRedisPubSub_Messaging(t *testing.T) {
	store := createTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	channel := "events:cluster"
	stream1, err := store.PubSub().Subscribe(ctx, []string{channel})
	if err != nil {
		t.Fatalf("Subscribe 1 failed: %v", err)
	}

	stream2, err := store.PubSub().Subscribe(ctx, []string{channel})
	if err != nil {
		t.Fatalf("Subscribe 2 failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	payload := []byte("test-event-payload")
	if err := store.PubSub().Publish(ctx, channel, payload); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	select {
	case msg := <-stream1:
		if msg.Channel != channel || string(msg.Payload) != string(payload) {
			t.Fatalf("unexpected message on stream1: %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for message on stream1")
	}

	select {
	case msg := <-stream2:
		if msg.Channel != channel || string(msg.Payload) != string(payload) {
			t.Fatalf("unexpected message on stream2: %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for message on stream2")
	}

	_ = store.PubSub().Unsubscribe(ctx, []string{channel})
}
