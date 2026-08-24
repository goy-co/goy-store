// Package goystore provides the unified persistence abstraction for the Goy Platform.
//
// Goy Store is not a database. It is an internal interface that defines standardized
// contracts for storage operations, allowing platform services to consume different
// backends through the same API, without knowing backend-specific details.
package goystore

import (
	"context"
	"time"
)

// GoyStore is the main interface aggregating all persistence contracts.
type GoyStore interface {
	KV() KVStore
	Relational() RelationalStore
	SortedSet() SortedSetStore
	PubSub() PubSubStore
	Blob() BlobStore
	Metrics() *Metrics
}

// KVStore defines the contract for ephemeral key-value operations.
type KVStore interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl *time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	SetIfNotExists(ctx context.Context, key string, value []byte, ttl *time.Duration) (bool, error)
}

// RelationalStore defines the contract for transactional CRUD operations.
type RelationalStore interface {
	Query(ctx context.Context, sql string, params []any) (Rows, error)
	Execute(ctx context.Context, sql string, params []any) (int64, error)
	Transaction(ctx context.Context, fn func(Tx) error) error
	Migrate(ctx context.Context, migrations []Migration) error
}

// Rows represents the result of a relational query.
type Rows interface {
	Columns() []string
	Next() bool
	Scan(dest ...any) error
	Close() error
}

// Tx represents a relational transaction.
type Tx interface {
	Query(ctx context.Context, sql string, params []any) (Rows, error)
	Execute(ctx context.Context, sql string, params []any) (int64, error)
}

// Migration represents a database migration.
type Migration struct {
	Version string
	UpSQL   string
	DownSQL string
}

// SortedSetStore defines the contract for temporal range queries.
type SortedSetStore interface {
	Add(ctx context.Context, set string, member string, score float64) error
	Remove(ctx context.Context, set string, member string) error
	RangeByScore(ctx context.Context, set string, min, max float64, limit *int) ([]ScoredMember, error)
	Count(ctx context.Context, set string) (int64, error)
	RemoveRange(ctx context.Context, set string, min, max float64) (int64, error)
	Score(ctx context.Context, set string, member string) (*float64, error)
}

// ScoredMember represents a member and its score in a sorted set.
type ScoredMember struct {
	Member string
	Score  float64
}

// PubSubStore defines the contract for event propagation.
type PubSubStore interface {
	Publish(ctx context.Context, channel string, message []byte) error
	Subscribe(ctx context.Context, channels []string) (<-chan Message, error)
	Unsubscribe(ctx context.Context, channels []string) error
}

// Message represents a pub/sub message.
type Message struct {
	Channel   string
	Payload   []byte
	Timestamp time.Time
}

// BlobStore defines the contract for object storage.
type BlobStore interface {
	Put(ctx context.Context, key string, data []byte, metadata *Metadata) error
	Get(ctx context.Context, key string) ([]byte, *Metadata, bool, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix *string) ([]string, error)
	PresignURL(ctx context.Context, key string, expiry time.Duration) (string, error)
}

// Metadata represents blob metadata.
type Metadata struct {
	ContentType string
	Custom      map[string]string
}