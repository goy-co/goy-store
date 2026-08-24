use anyhow::Result;
use goy_store::{GoyStore, config::StoreConfig};
use std::env;

pub fn get_redis_url() -> String {
    env::var("REDIS_URL").unwrap_or_else(|_| "redis://127.0.0.1:6379".to_string())
}

pub fn get_postgres_url() -> String {
    env::var("DATABASE_URL").unwrap_or_else(|_| {
        "postgres://test:test@127.0.0.1:5432/goy_store_test?sslmode=disable".to_string()
    })
}

pub async fn create_test_store() -> Result<GoyStore> {
    let mut config = StoreConfig::default();

    // KV
    config.kv.backend = "redis".to_string();
    config.kv.url = Some(get_redis_url());

    // Relational
    config.relational.backend = "postgres".to_string();
    config.relational.url = Some(get_postgres_url());

    // Sorted Set
    config.sorted_set.backend = "redis".to_string();
    config.sorted_set.url = Some(get_redis_url());

    // PubSub
    config.pubsub.backend = "redis".to_string();
    config.pubsub.url = Some(get_redis_url());

    // Blob (Local)
    let temp_dir = tempfile::tempdir()?;
    config.blob.backend = "local".to_string();
    config.blob.path = Some(temp_dir.path().to_string_lossy().to_string());

    GoyStore::from_config(&config).await
}

pub async fn flush_redis() -> Result<()> {
    let client = redis::Client::open(get_redis_url())?;
    let mut conn = client.get_multiplexed_async_connection().await?;
    let () = redis::cmd("FLUSHALL").query_async(&mut conn).await?;
    Ok(())
}

pub async fn reset_postgres_tables() -> Result<()> {
    let pool = sqlx::PgPool::connect(&get_postgres_url()).await?;
    sqlx::query("DROP TABLE IF EXISTS schema_migrations CASCADE;")
        .execute(&pool)
        .await?;
    sqlx::query("DROP TABLE IF EXISTS users CASCADE;")
        .execute(&pool)
        .await?;
    sqlx::query("DROP TABLE IF EXISTS nodes CASCADE;")
        .execute(&pool)
        .await?;
    Ok(())
}

pub fn get_minio_endpoint() -> String {
    env::var("MINIO_ENDPOINT").unwrap_or_else(|_| "http://127.0.0.1:9000".to_string())
}

pub fn get_minio_access_key() -> String {
    env::var("MINIO_ACCESS_KEY").unwrap_or_else(|_| "minioadmin".to_string())
}

pub fn get_minio_secret_key() -> String {
    env::var("MINIO_SECRET_KEY").unwrap_or_else(|_| "minioadmin".to_string())
}

pub fn get_minio_bucket() -> String {
    env::var("MINIO_BUCKET").unwrap_or_else(|_| "goy-store-test".to_string())
}

pub async fn setup_minio_bucket() -> Result<goy_store::blob::S3BlobStore> {
    let mut blob_config = goy_store::config::BlobConfig::default();
    blob_config.backend = "s3".to_string();
    blob_config.endpoint = Some(get_minio_endpoint());
    blob_config.region = Some("us-east-1".to_string());
    blob_config.bucket = Some(get_minio_bucket());
    blob_config.access_key = Some(get_minio_access_key());
    blob_config.secret_key = Some(get_minio_secret_key());
    blob_config.force_path_style = Some(true);

    let store = goy_store::blob::S3BlobStore::new(&blob_config).await?;

    // Create bucket if it doesn't exist
    let sdk_config = aws_config::defaults(aws_config::BehaviorVersion::latest())
        .region(aws_sdk_s3::config::Region::new("us-east-1"))
        .credentials_provider(aws_credential_types::Credentials::new(
            get_minio_access_key(),
            get_minio_secret_key(),
            None,
            None,
            "minio",
        ))
        .load()
        .await;

    let client = aws_sdk_s3::Client::from_conf(
        aws_sdk_s3::config::Builder::from(&sdk_config)
            .endpoint_url(get_minio_endpoint())
            .force_path_style(true)
            .build(),
    );

    let _ = client
        .create_bucket()
        .bucket(get_minio_bucket())
        .send()
        .await;

    Ok(store)
}
