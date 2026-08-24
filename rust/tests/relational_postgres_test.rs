use super::common::*;
use goy_store::relational::Migration;

#[tokio::test]
async fn test_postgres_relational_queries_and_migrations() {
    reset_postgres_tables().await.expect("failed to reset tables");
    let store = create_test_store().await.expect("failed to create store");

    // 1. Run migrations
    let migrations = vec![
        Migration {
            version: "202608240001".to_string(),
            up_sql: r#"
                CREATE TABLE users (
                    id VARCHAR(64) PRIMARY KEY,
                    name VARCHAR(255) NOT NULL,
                    age INT NOT NULL
                );
            "#.to_string(),
            down_sql: "DROP TABLE users;".to_string(),
        },
        Migration {
            version: "202608240002".to_string(),
            up_sql: r#"
                CREATE TABLE nodes (
                    id VARCHAR(64) PRIMARY KEY,
                    region VARCHAR(64) NOT NULL
                );
            "#.to_string(),
            down_sql: "DROP TABLE nodes;".to_string(),
        },
    ];

    store.relational.migrate(&migrations).await.unwrap();

    // Re-running migrations should be idempotent
    store.relational.migrate(&migrations).await.unwrap();

    // 2. Execute insert queries
    let affected = store.relational.execute(
        "INSERT INTO users (id, name, age) VALUES ('user-1', 'Alice', 30)",
        &[],
    ).await.unwrap();
    assert_eq!(affected, 1);

    store.relational.execute(
        "INSERT INTO users (id, name, age) VALUES ('user-2', 'Bob', 25)",
        &[],
    ).await.unwrap();

    // 3. Query
    let rows = store.relational.query(
        "SELECT id, name, age FROM users ORDER BY age ASC",
        &[],
    ).await.unwrap();

    assert_eq!(rows.columns, vec!["id", "name", "age"]);
    assert_eq!(rows.rows.len(), 2);
    assert_eq!(String::from_utf8_lossy(&rows.rows[0][1]), "Bob");
    assert_eq!(String::from_utf8_lossy(&rows.rows[1][1]), "Alice");
}
