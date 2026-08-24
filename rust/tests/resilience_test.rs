use goy_store::config::ResilienceConfig;
use goy_store::metrics::StoreMetrics;
use goy_store::resilience::{CircuitBreaker, ResilientExecutor};
use prometheus::Registry;
use std::sync::Arc;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::time::Duration;

#[tokio::test]
async fn test_resilience_circuit_breaker_and_retries() {
    let registry = Registry::new();
    let metrics = Arc::new(StoreMetrics::new(&registry).unwrap());

    let config = ResilienceConfig {
        max_retries: 3,
        base_backoff_ms: 10,
        circuit_breaker_threshold: 3,
        circuit_breaker_reset_seconds: 1,
        operation_timeout_seconds: 2,
    };

    let executor = ResilientExecutor::new(config, metrics.clone(), "test", "mock");

    // 1. Retry test
    let attempts = Arc::new(AtomicUsize::new(0));
    let attempts_clone = attempts.clone();

    let res = executor
        .execute("flaky_op", || {
            let att = attempts_clone.clone();
            async move {
                let current = att.fetch_add(1, Ordering::SeqCst);
                if current < 2 {
                    Err(anyhow::anyhow!("transient error"))
                } else {
                    Ok(42)
                }
            }
        })
        .await;

    assert!(res.is_ok());
    assert_eq!(res.unwrap(), 42);
    assert_eq!(attempts.load(Ordering::SeqCst), 3);

    // 2. Circuit breaker opening after threshold failures
    let cb = CircuitBreaker::new(3, Duration::from_millis(200));
    assert!(cb.can_execute().await.is_ok());

    cb.on_failure().await;
    cb.on_failure().await;
    cb.on_failure().await;

    // Circuit should be open
    assert!(cb.can_execute().await.is_err());

    // Wait for reset timeout
    tokio::time::sleep(Duration::from_millis(250)).await;

    // Probe should be allowed (half-open)
    assert!(cb.can_execute().await.is_ok());

    // On success, circuit closes
    cb.on_success().await;
    assert!(cb.can_execute().await.is_ok());
}
