use anyhow::Result;
use goy_store::{config::StoreConfig, GoyStore};
use std::env;

pub fn get_redis_url() -> String {
    env::var("REDIS_URL").unwrap_or_else(|_| "redis://127.0.0.1:6379".to_string())
}

pub fn get_postgres_url() -> String {
    env::var("DATABASE_URL").unwrap_or_else(|_| "postgres://test:test@127.0.0.1:5432/goy_store_test?sslmode=disable".to_string())
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
    sqlx::query("DROP TABLE IF EXISTS schema_migrations CASCADE;").execute(&pool).await?;
    sqlx::query("DROP TABLE IF EXISTS users CASCADE;").execute(&pool).await?;
    sqlx::query("DROP TABLE IF EXISTS nodes CASCADE;").execute(&pool).await?;
    Ok(())
}
