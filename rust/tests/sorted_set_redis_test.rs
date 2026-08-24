use super::common::*;

#[tokio::test]
async fn test_redis_sorted_set_heartbeat_lifecycle() {
    flush_redis().await.expect("failed to flush redis");
    let store = create_test_store().await.expect("failed to create store");

    let set_name = "cluster:heartbeats";

    // 1. Add heartbeats with unix timestamps
    store
        .sorted_set
        .add(set_name, "node-1", 1000.0)
        .await
        .unwrap();
    store
        .sorted_set
        .add(set_name, "node-2", 1050.0)
        .await
        .unwrap();
    store
        .sorted_set
        .add(set_name, "node-3", 1100.0)
        .await
        .unwrap();

    // 2. Count
    let total = store.sorted_set.count(set_name).await.unwrap();
    assert_eq!(total, 3);

    // 3. Score
    let score = store.sorted_set.score(set_name, "node-2").await.unwrap();
    assert_eq!(score, Some(1050.0));

    let score_missing = store
        .sorted_set
        .score(set_name, "node-unknown")
        .await
        .unwrap();
    assert_eq!(score_missing, None);

    // 4. Range by score (find active nodes between 1040 and 1110)
    let active = store
        .sorted_set
        .range_by_score(set_name, 1040.0, 1110.0, None)
        .await
        .unwrap();
    assert_eq!(active.len(), 2);
    assert_eq!(active[0].0, "node-2");
    assert_eq!(active[1].0, "node-3");

    // 5. Remove expired nodes (older than 1020)
    let removed = store
        .sorted_set
        .remove_range(set_name, 0.0, 1020.0)
        .await
        .unwrap();
    assert_eq!(removed, 1);

    let remaining = store.sorted_set.count(set_name).await.unwrap();
    assert_eq!(remaining, 2);

    // 6. Explicit remove
    store.sorted_set.remove(set_name, "node-3").await.unwrap();
    assert_eq!(store.sorted_set.count(set_name).await.unwrap(), 1);
}
