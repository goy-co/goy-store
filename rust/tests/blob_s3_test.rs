use super::common::*;
use goy_store::blob::{BlobStore, Metadata};
use std::collections::HashMap;
use std::time::Duration;

#[tokio::test]
async fn test_s3_blob_store_operations_against_minio() {
    let store = setup_minio_bucket().await.expect("failed to setup minio bucket");

    let key = "releases/v1.0.0/app-binary.tar.gz";
    let payload = vec![0xCA, 0xFE, 0xBA, 0xBE, 0x01, 0x02, 0x03, 0x04];
    let mut custom = HashMap::new();
    custom.insert("version".to_string(), "1.0.0".to_string());
    custom.insert("commit".to_string(), "abcdef".to_string());

    let meta = Metadata {
        content_type: Some("application/gzip".to_string()),
        custom,
    };

    // 1. Put
    store.put(key, &payload, Some(meta.clone())).await.unwrap();

    // 2. Get
    let fetched = store.get(key).await.unwrap();
    assert!(fetched.is_some(), "expected object to exist in S3");
    let (data, fetched_meta) = fetched.unwrap();
    assert_eq!(data, payload);
    assert_eq!(fetched_meta.content_type, Some("application/gzip".to_string()));
    assert_eq!(fetched_meta.custom.get("version").map(|s| s.as_str()), Some("1.0.0"));

    // 3. List
    let list = store.list(Some("releases")).await.unwrap();
    assert_eq!(list, vec!["releases/v1.0.0/app-binary.tar.gz"]);

    // 4. Presign URL
    let presigned_url = store.presign_url(key, Duration::from_secs(300)).await.unwrap();
    assert!(presigned_url.contains("http://"));
    assert!(presigned_url.contains(key));

    // 5. Health check
    let health = store.is_healthy().await.unwrap();
    assert_eq!(health.state, goy_store::health::HealthState::Healthy);
    assert_eq!(health.contract, "blob");
    assert_eq!(health.backend, "s3");

    // 6. Delete
    store.delete(key).await.unwrap();
    let after_delete = store.get(key).await.unwrap();
    assert!(after_delete.is_none());
}
