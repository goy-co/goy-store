use super::common::*;
use goy_store::health::HealthState;

#[tokio::test]
async fn test_health_check_backends_healthy() {
    let store = create_test_store().await.expect("failed to create store");

    // 1. Check individual contracts
    let kv_health = store.kv.is_healthy().await.unwrap();
    assert_eq!(kv_health.state, HealthState::Healthy);
    assert_eq!(kv_health.contract, "kv");
    assert_eq!(kv_health.backend, "redis");

    let rel_health = store.relational.is_healthy().await.unwrap();
    assert_eq!(rel_health.state, HealthState::Healthy);
    assert_eq!(rel_health.contract, "relational");
    assert_eq!(rel_health.backend, "postgres");

    let ss_health = store.sorted_set.is_healthy().await.unwrap();
    assert_eq!(ss_health.state, HealthState::Healthy);
    assert_eq!(ss_health.backend, "redis");

    let ps_health = store.pubsub.is_healthy().await.unwrap();
    assert_eq!(ps_health.state, HealthState::Healthy);
    assert_eq!(ps_health.backend, "redis");

    let blob_health = store.blob.is_healthy().await.unwrap();
    assert_eq!(blob_health.state, HealthState::Healthy);
    assert_eq!(blob_health.backend, "local");

    // 2. Consolidated health
    let consolidated = store.health_check().await;
    assert_eq!(consolidated.state, HealthState::Healthy);
    assert_eq!(consolidated.contracts.len(), 5);
}
