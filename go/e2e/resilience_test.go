//go:build e2e

package e2e

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	goystore "github.com/goy-co/goy-store/go"
)

func TestResilience_CircuitBreakerAndRetries(t *testing.T) {
	metrics := goystore.DefaultMetrics()
	cfg := goystore.ResilienceConfig{
		MaxRetries:                 3,
		BaseBackoffMS:              10,
		CircuitBreakerThreshold:    3,
		CircuitBreakerResetSeconds: 1,
		OperationTimeoutSeconds:    2,
	}

	exec := goystore.NewResilientExecutor(cfg, metrics, "test", "mock")

	// 1. Retry test
	var attempts int32
	ctx := context.Background()

	res, err := goystore.ExecuteWithResilience(exec, ctx, "flaky_op", func(c context.Context) (int, error) {
		curr := atomic.AddInt32(&attempts, 1)
		if curr < 3 {
			return 0, errors.New("transient error")
		}
		return 42, nil
	})

	if err != nil || res != 42 {
		t.Fatalf("expected success with 42, got %v, err=%v", res, err)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Fatalf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}

	// 2. Circuit breaker
	cb := goystore.NewCircuitBreaker(3, 200*time.Millisecond)
	if err := cb.CanExecute(); err != nil {
		t.Fatalf("cb should be closed initially: %v", err)
	}

	cb.OnFailure()
	cb.OnFailure()
	cb.OnFailure()

	// Circuit should be open
	if err := cb.CanExecute(); err == nil {
		t.Fatalf("cb should be open after 3 failures")
	}

	// Wait for reset timeout
	time.Sleep(250 * time.Millisecond)

	// Half-open probe
	if err := cb.CanExecute(); err != nil {
		t.Fatalf("cb should allow probe after reset timeout: %v", err)
	}

	cb.OnSuccess()
	if err := cb.CanExecute(); err != nil {
		t.Fatalf("cb should be closed after success: %v", err)
	}
}
