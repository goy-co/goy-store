package goystore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRelationalStore implements the RelationalStore contract using PostgreSQL (pgxpool).
type PostgresRelationalStore struct {
	pool *pgxpool.Pool
}

// NewPostgresRelationalStore creates a new PostgresRelationalStore from RelationalConfig.
func NewPostgresRelationalStore(ctx context.Context, cfg RelationalConfig) (*PostgresRelationalStore, error) {
	url := cfg.URL
	if url == "" {
		url = "postgres://postgres:postgres@127.0.0.1:5432/goy"
	}

	poolCfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres url: %w", err)
	}

	if cfg.PoolSize > 0 {
		poolCfg.MaxConns = int32(cfg.PoolSize)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres pool: %w", err)
	}

	return &PostgresRelationalStore{pool: pool}, nil
}

// NewPostgresRelationalStoreWithPool creates a PostgresRelationalStore with an existing pool.
func NewPostgresRelationalStoreWithPool(pool *pgxpool.Pool) *PostgresRelationalStore {
	return &PostgresRelationalStore{pool: pool}
}

// Close closes the underlying pool.
func (p *PostgresRelationalStore) Close() {
	if p.pool != nil {
		p.pool.Close()
	}
}

// Query executes a query returning multiple rows.
func (p *PostgresRelationalStore) Query(ctx context.Context, sql string, params []any) (Rows, error) {
	pgxRows, err := p.pool.Query(ctx, sql, params...)
	if err != nil {
		return nil, err
	}
	return &pgxRowsWrapper{rows: pgxRows}, nil
}

// Execute executes a query returning the number of affected rows.
func (p *PostgresRelationalStore) Execute(ctx context.Context, sql string, params []any) (int64, error) {
	tag, err := p.pool.Exec(ctx, sql, params...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// Transaction executes a function within an ACID transaction.
func (p *PostgresRelationalStore) Transaction(ctx context.Context, fn func(Tx) error) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	wrappedTx := &pgxTxWrapper{tx: tx}
	if err := fn(wrappedTx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Migrate applies migrations inside transactions, tracking state via schema_migrations.
func (p *PostgresRelationalStore) Migrate(ctx context.Context, migrations []Migration) error {
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`
	if _, err := p.pool.Exec(ctx, createTableSQL); err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	for _, m := range migrations {
		var exists bool
		err := p.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", m.Version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration %s: %w", m.Version, err)
		}

		if !exists {
			err = p.Transaction(ctx, func(tx Tx) error {
				if _, err := tx.Execute(ctx, m.UpSQL, nil); err != nil {
					return fmt.Errorf("failed executing migration up_sql for %s: %w", m.Version, err)
				}
				if _, err := tx.Execute(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", []any{m.Version}); err != nil {
					return fmt.Errorf("failed recording migration %s: %w", m.Version, err)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// --- Wrappers ---

type pgxRowsWrapper struct {
	rows pgx.Rows
}

func (r *pgxRowsWrapper) Columns() []string {
	fields := r.rows.FieldDescriptions()
	cols := make([]string, len(fields))
	for i, f := range fields {
		cols[i] = string(f.Name)
	}
	return cols
}

func (r *pgxRowsWrapper) Next() bool {
	return r.rows.Next()
}

func (r *pgxRowsWrapper) Scan(dest ...any) error {
	return r.rows.Scan(dest...)
}

func (r *pgxRowsWrapper) Close() error {
	r.rows.Close()
	return r.rows.Err()
}

type pgxTxWrapper struct {
	tx pgx.Tx
}

func (t *pgxTxWrapper) Query(ctx context.Context, sql string, params []any) (Rows, error) {
	rows, err := t.tx.Query(ctx, sql, params...)
	if err != nil {
		return nil, err
	}
	return &pgxRowsWrapper{rows: rows}, nil
}

func (t *pgxTxWrapper) Execute(ctx context.Context, sql string, params []any) (int64, error) {
	tag, err := t.tx.Exec(ctx, sql, params...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
