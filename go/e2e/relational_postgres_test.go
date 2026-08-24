//go:build e2e

package e2e

import (
	"context"
	"testing"

	goystore "github.com/goy-co/goy-store/go"
)

func TestPostgresRelational_QueriesAndMigrations(t *testing.T) {
	resetPostgresTables(t)
	store := createTestStore(t)
	ctx := context.Background()

	// 1. Run migrations
	migrations := []goystore.Migration{
		{
			Version: "202608240001",
			UpSQL: `CREATE TABLE users (
				id VARCHAR(64) PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				age INT NOT NULL
			);`,
			DownSQL: "DROP TABLE users;",
		},
		{
			Version: "202608240002",
			UpSQL: `CREATE TABLE nodes (
				id VARCHAR(64) PRIMARY KEY,
				region VARCHAR(64) NOT NULL
			);`,
			DownSQL: "DROP TABLE nodes;",
		},
	}

	if err := store.Relational().Migrate(ctx, migrations); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	// Idempotency test
	if err := store.Relational().Migrate(ctx, migrations); err != nil {
		t.Fatalf("Idempotent Migrate failed: %v", err)
	}

	// 2. Execute insert queries
	aff, err := store.Relational().Execute(ctx, "INSERT INTO users (id, name, age) VALUES ($1, $2, $3)", []any{"user-1", "Alice", 30})
	if err != nil || aff != 1 {
		t.Fatalf("Execute insert failed: aff=%d, err=%v", aff, err)
	}

	_, err = store.Relational().Execute(ctx, "INSERT INTO users (id, name, age) VALUES ($1, $2, $3)", []any{"user-2", "Bob", 25})
	if err != nil {
		t.Fatalf("Execute insert 2 failed: %v", err)
	}

	// 3. Query
	rows, err := store.Relational().Query(ctx, "SELECT id, name, age FROM users ORDER BY age ASC", nil)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	type User struct {
		ID   string
		Name string
		Age  int32
	}

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Age); err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		users = append(users, u)
	}

	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	if users[0].Name != "Bob" || users[1].Name != "Alice" {
		t.Fatalf("unexpected users order/content: %+v", users)
	}
}
