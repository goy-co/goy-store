//! Relational Store Contract
//!
//! Relational operations with ACID transactions. Used for node registries, credentials,
//! persistent configuration, audit logs.

use anyhow::Result;
use async_trait::async_trait;

#[derive(Clone)]
pub struct Param {
    // Simplified parameter representation
    pub value: Vec<u8>,
}

pub struct Rows {
    pub columns: Vec<String>,
    pub rows: Vec<Vec<Vec<u8>>>,
}

pub struct Transaction;

#[derive(Clone)]
pub struct Migration {
    pub version: String,
    pub up_sql: String,
    pub down_sql: String,
}

#[async_trait]
pub trait RelationalStore: Send + Sync {
    async fn query(&self, sql: &str, params: &[Param]) -> Result<Rows>;
    async fn execute(&self, sql: &str, params: &[Param]) -> Result<u64>;
    async fn migrate(&self, migrations: &[Migration]) -> Result<()>;
    async fn is_healthy(&self) -> Result<crate::health::HealthStatus>;
}

/// In-memory implementation of RelationalStore for testing and local development.
#[derive(Default)]
pub struct MemoryRelationalStore;

#[async_trait]
impl RelationalStore for MemoryRelationalStore {
    async fn query(&self, _sql: &str, _params: &[Param]) -> Result<Rows> {
        Ok(Rows {
            columns: vec![],
            rows: vec![],
        })
    }

    async fn execute(&self, _sql: &str, _params: &[Param]) -> Result<u64> {
        Ok(0)
    }

    async fn migrate(&self, _migrations: &[Migration]) -> Result<()> {
        Ok(())
    }

    async fn is_healthy(&self) -> Result<crate::health::HealthStatus> {
        Ok(crate::health::HealthStatus::healthy("relational", "memory", 0))
    }
}

#[cfg(feature = "sqlx-backend")]
pub struct PostgresRelationalStore {
    pool: sqlx::PgPool,
}

#[cfg(feature = "sqlx-backend")]
impl PostgresRelationalStore {
    pub async fn new(url: &str) -> Result<Self> {
        let pool = sqlx::postgres::PgPoolOptions::new()
            .max_connections(20)
            .connect(url)
            .await?;
        Ok(Self { pool })
    }

    pub fn from_pool(pool: sqlx::PgPool) -> Self {
        Self { pool }
    }
}

#[cfg(feature = "sqlx-backend")]
#[async_trait]
impl RelationalStore for PostgresRelationalStore {
    async fn query(&self, sql: &str, _params: &[Param]) -> Result<Rows> {
        use sqlx::Row;
        let rows = sqlx::query(sql).fetch_all(&self.pool).await?;
        let mut result_columns = Vec::new();
        let mut result_rows = Vec::new();

        if let Some(first_row) = rows.first() {
            use sqlx::Column;
            for col in first_row.columns() {
                result_columns.push(col.name().to_string());
            }
        }

        for row in rows {
            let mut row_data = Vec::new();
            for i in 0..result_columns.len() {
                // Fetch raw bytes or string representation
                let val: Option<Vec<u8>> = row.try_get(i).ok();
                row_data.push(val.unwrap_or_default());
            }
            result_rows.push(row_data);
        }

        Ok(Rows {
            columns: result_columns,
            rows: result_rows,
        })
    }

    async fn execute(&self, sql: &str, _params: &[Param]) -> Result<u64> {
        let res = sqlx::query(sql).execute(&self.pool).await?;
        Ok(res.rows_affected())
    }

    async fn migrate(&self, migrations: &[Migration]) -> Result<()> {
        sqlx::query(
            r#"
            CREATE TABLE IF NOT EXISTS schema_migrations (
                version VARCHAR(255) PRIMARY KEY,
                applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
            );
            "#,
        )
        .execute(&self.pool)
        .await?;

        for migration in migrations {
            let applied: Option<(String,)> = sqlx::query_as(
                "SELECT version FROM schema_migrations WHERE version = $1",
            )
            .bind(&migration.version)
            .fetch_optional(&self.pool)
            .await?;

            if applied.is_none() {
                let mut tx = self.pool.begin().await?;
                sqlx::query(&migration.up_sql).execute(&mut *tx).await?;
                sqlx::query("INSERT INTO schema_migrations (version) VALUES ($1)")
                    .bind(&migration.version)
                    .execute(&mut *tx)
                    .await?;
                tx.commit().await?;
            }
        }

        Ok(())
    }

    async fn is_healthy(&self) -> Result<crate::health::HealthStatus> {
        let start = std::time::Instant::now();
        let timeout_fut = tokio::time::timeout(
            std::time::Duration::from_secs(3),
            sqlx::query("SELECT 1").execute(&self.pool),
        );

        match timeout_fut.await {
            Ok(Ok(_)) => {
                let latency = start.elapsed().as_millis() as u64;
                Ok(crate::health::HealthStatus::healthy("relational", "postgres", latency))
            }
            Ok(Err(e)) => {
                let latency = start.elapsed().as_millis() as u64;
                Ok(crate::health::HealthStatus::unhealthy("relational", "postgres", &e.to_string(), latency))
            }
            Err(_) => {
                let latency = start.elapsed().as_millis() as u64;
                Ok(crate::health::HealthStatus::unhealthy("relational", "postgres", "health check timed out (3s)", latency))
            }
        }
    }
}