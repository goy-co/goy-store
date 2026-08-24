use super::common::*;
use std::time::Duration;

#[tokio::test]
async fn test_redis_kv_crud_and_ttl() {
    flush_redis().await.expect("failed to flush redis");
    let store = create_test_store().await.expect("failed to create store");

    // 1. Set & Get
    store.kv.set("node:1:status", b"active", None).await.unwrap();
    let val = store.kv.get("node:1:status").await.unwrap();
    assert_eq!(val, Some(b"active".to_vec()));

    // 2. Exists
    let exists = store.kv.exists("node:1:status").await.unwrap();
    assert!(exists);

    // 3. SetIfNotExists
    let set_nx = store.kv.set_if_not_exists("node:1:status", b"duplicate", None).await.unwrap();
    assert!(!set_nx, "set_if_not_exists should return false for existing key");

    let set_nx_new = store.kv.set_if_not_exists("node:2:status", b"standby", None).await.unwrap();
    assert!(set_nx_new, "set_if_not_exists should return true for new key");

    // 4. TTL expiration
    store.kv.set("ephemeral:key", b"temp", Some(Duration::from_millis(300))).await.unwrap();
    assert_eq!(store.kv.get("ephemeral:key").await.unwrap(), Some(b"temp".to_vec()));
    tokio::time::sleep(Duration::from_millis(400)).await;
    assert_eq!(store.kv.get("ephemeral:key").await.unwrap(), None);

    // 5. Delete
    store.kv.delete("node:1:status").await.unwrap();
    assert!(!store.kv.exists("node:1:status").await.unwrap());
}

#[tokio::test]
async fn test_redis_kv_concurrency() {
    flush_redis().await.expect("failed to flush redis");
    let store = create_test_store().await.expect("failed to create store");

    let mut handles = Vec::new();
    for i in 0..20 {
        let kv = store.kv.clone();
        handles.push(tokio::spawn(async move {
            let key = format!("concurrent:key:{}", i);
            let val = format!("val-{}", i);
            kv.set(&key, val.as_bytes(), None).await.unwrap();
            let read_back = kv.get(&key).await.unwrap();
            assert_eq!(read_back, Some(val.into_bytes()));
        }));
    }

    for h in handles {
        h.await.unwrap();
    }
}
