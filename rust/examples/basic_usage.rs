use goy_store::{GoyStore, config::StoreConfig};
use std::time::Duration;

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    println!("🚀 Starting Goy Store Basic Usage Example (Rust)");

    // 1. Initialize GoyStore with default in-memory configuration
    let config = StoreConfig::default();
    let store = GoyStore::from_config(&config).await?;

    // 2. KV Store Operations
    println!("\n📦 [KV Store]");
    store
        .kv
        .set("node:1:status", b"active", Some(Duration::from_secs(60)))
        .await?;
    if let Some(val) = store.kv.get("node:1:status").await? {
        println!(
            "  -> Fetched key 'node:1:status': {}",
            String::from_utf8_lossy(&val)
        );
    }

    // 3. Sorted Set Operations
    println!("\n⏱️ [Sorted Set Store]");
    store
        .sorted_set
        .add("nodes:heartbeat", "node-1", 1000.0)
        .await?;
    store
        .sorted_set
        .add("nodes:heartbeat", "node-2", 1050.0)
        .await?;
    let active_nodes = store
        .sorted_set
        .range_by_score("nodes:heartbeat", 900.0, 1100.0, None)
        .await?;
    println!("  -> Active nodes count: {}", active_nodes.len());
    for (node, score) in active_nodes {
        println!("     - {} (score: {})", node, score);
    }

    // 4. Blob Store Operations
    println!("\n📁 [Blob Store]");
    store
        .blob
        .put(
            "configs/cluster.json",
            b"{\"cluster\": \"goy-alpha\"}",
            None,
        )
        .await?;
    let blob_url = store
        .blob
        .presign_url("configs/cluster.json", Duration::from_secs(300))
        .await?;
    println!("  -> Presigned URL: {}", blob_url);

    // 5. Health Check
    println!("\n🩺 [Health Check]");
    let health = store.health_check().await;
    println!("  -> Overall Status: {:?}", health.state);
    for (contract, status) in health.contracts {
        println!(
            "     - {}: {:?} (backend: {}, latency: {}ms)",
            contract, status.state, status.backend, status.latency_ms
        );
    }

    println!("\n✅ Example completed successfully.");
    Ok(())
}
