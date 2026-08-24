package goystore

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

// Config represents the Goy Store configuration.
type Config struct {
	KV          KVConfig          `toml:"kv"`
	Relational  RelationalConfig  `toml:"relational"`
	SortedSet   SortedSetConfig   `toml:"sorted_set"`
	PubSub      PubSubConfig      `toml:"pubsub"`
	Blob        BlobConfig        `toml:"blob"`
}

// KVConfig represents the configuration for the KV store.
type KVConfig struct {
	Backend   string `toml:"backend"`
	URL       string `toml:"url,omitempty"`
	PoolSize  int    `toml:"pool_size,omitempty"`
	TimeoutMS int    `toml:"timeout_ms,omitempty"`
}

// RelationalConfig represents the configuration for the relational store.
type RelationalConfig struct {
	Backend    string `toml:"backend"`
	URL        string `toml:"url,omitempty"`
	PoolSize   int    `toml:"pool_size,omitempty"`
	MaxRetries int    `toml:"max_retries,omitempty"`
}

// SortedSetConfig represents the configuration for the sorted set store.
type SortedSetConfig struct {
	Backend string `toml:"backend"`
	URL     string `toml:"url,omitempty"`
}

// PubSubConfig represents the configuration for the pub/sub store.
type PubSubConfig struct {
	Backend string `toml:"backend"`
	URL     string `toml:"url,omitempty"`
}

// BlobConfig represents the configuration for the blob store.
type BlobConfig struct {
	Backend  string `toml:"backend"`
	Endpoint string `toml:"endpoint,omitempty"`
	Bucket   string `toml:"bucket,omitempty"`
	Region   string `toml:"region,omitempty"`
	Path     string `toml:"path,omitempty"`
}

// LoadConfig loads the configuration from a TOML file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// DefaultConfig returns a default configuration suitable for local development.
func DefaultConfig() *Config {
	return &Config{
		KV: KVConfig{
			Backend: "memory",
		},
		Relational: RelationalConfig{
			Backend: "memory",
		},
		SortedSet: SortedSetConfig{
			Backend: "memory",
		},
		PubSub: PubSubConfig{
			Backend: "memory",
		},
		Blob: BlobConfig{
			Backend: "memory",
		},
	}
}