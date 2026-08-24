use super::common::*;
use goy_store::blob::Metadata;
use std::collections::HashMap;
use std::time::Duration;

#[tokio::test]
async fn test_local_blob_store_operations() {
    let store = create_test_store().await.expect("failed to create store");

    let key = "configs/node-1/app.json";
    let payload = vec![0xAB; 64 * 1024]; // 64KB
    let mut custom = HashMap::new();
    custom.insert("author".to_string(), "goy-team".to_string());

    let meta = Metadata {
        content_type: Some("application/json".to_string()),
        custom,
    };

    // 1. Put
    store
        .blob
        .put(key, &payload, Some(meta.clone()))
        .await
        .unwrap();

    // 2. Get
    let fetched = store.blob.get(key).await.unwrap();
    assert!(fetched.is_some());
    let (data, fetched_meta) = fetched.unwrap();
    assert_eq!(data, payload);
    assert_eq!(
        fetched_meta.content_type,
        Some("application/json".to_string())
    );
    assert_eq!(fetched_meta.custom.get("author").unwrap(), "goy-team");

    // 3. List with prefix
    let list = store.blob.list(Some("configs")).await.unwrap();
    assert_eq!(list, vec!["configs/node-1/app.json"]);

    // 4. Presign URL
    let url = store
        .blob
        .presign_url(key, Duration::from_secs(60))
        .await
        .unwrap();
    assert!(url.starts_with("file://"));

    // 5. Delete
    store.blob.delete(key).await.unwrap();
    let after_del = store.blob.get(key).await.unwrap();
    assert!(after_del.is_none());
}
