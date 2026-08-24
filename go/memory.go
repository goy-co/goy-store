package goystore

import (
	"context"
	"sort"
	"sync"
	"time"
)

// MemoryStore is an in-memory implementation of GoyStore for testing and local development.
type MemoryStore struct {
	kv         *memoryKV
	relational *memoryRelational
	sortedSet  *memorySortedSet
	pubsub     *memoryPubSub
	blob       *memoryBlob
	metrics    *Metrics
}

// NewMemoryStore creates a new in-memory GoyStore.
func NewMemoryStore() GoyStore {
	return &MemoryStore{
		kv:         &memoryKV{data: make(map[string][]byte)},
		relational: &memoryRelational{},
		sortedSet:  &memorySortedSet{sets: make(map[string]map[string]float64)},
		pubsub:     &memoryPubSub{subscribers: make(map[string]map[chan Message]struct{})},
		blob:       &memoryBlob{blobs: make(map[string]blobData)},
		metrics:    DefaultMetrics(),
	}
}

func (s *MemoryStore) KV() KVStore                 { return s.kv }
func (s *MemoryStore) Relational() RelationalStore { return s.relational }
func (s *MemoryStore) SortedSet() SortedSetStore   { return s.sortedSet }
func (s *MemoryStore) PubSub() PubSubStore         { return s.pubsub }
func (s *MemoryStore) Blob() BlobStore             { return s.blob }
func (s *MemoryStore) Metrics() *Metrics           { return s.metrics }

func (s *MemoryStore) HealthCheck(ctx context.Context) ConsolidatedHealth {
	kvH, _ := s.kv.IsHealthy(ctx)
	relH, _ := s.relational.IsHealthy(ctx)
	ssH, _ := s.sortedSet.IsHealthy(ctx)
	psH, _ := s.pubsub.IsHealthy(ctx)
	blobH, _ := s.blob.IsHealthy(ctx)

	contracts := map[string]HealthStatus{
		"kv":         *kvH,
		"relational": *relH,
		"sorted_set": *ssH,
		"pubsub":     *psH,
		"blob":       *blobH,
	}

	return ConsolidatedHealth{
		State:     HealthHealthy,
		Contracts: contracts,
	}
}

// --- KV Store ---

type memoryKV struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func (m *memoryKV) Get(_ context.Context, key string) ([]byte, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.data[key]
	return val, ok, nil
}

func (m *memoryKV) Set(_ context.Context, key string, value []byte, _ *time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
	return nil
}

func (m *memoryKV) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *memoryKV) Exists(_ context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.data[key]
	return ok, nil
}

func (m *memoryKV) SetIfNotExists(_ context.Context, key string, value []byte, _ *time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.data[key]; ok {
		return false, nil
	}
	m.data[key] = value
	return true, nil
}

func (m *memoryKV) IsHealthy(_ context.Context) (*HealthStatus, error) {
	return &HealthStatus{
		Contract:  "kv",
		Backend:   "memory",
		State:     HealthHealthy,
		LatencyMS: 0,
	}, nil
}

// --- Relational Store ---

type memoryRelational struct{}

func (m *memoryRelational) Query(_ context.Context, _ string, _ []any) (Rows, error) {
	return nil, nil
}

func (m *memoryRelational) Execute(_ context.Context, _ string, _ []any) (int64, error) {
	return 0, nil
}

func (m *memoryRelational) Transaction(_ context.Context, _ func(Tx) error) error {
	return nil
}

func (m *memoryRelational) Migrate(_ context.Context, _ []Migration) error {
	return nil
}

func (m *memoryRelational) IsHealthy(_ context.Context) (*HealthStatus, error) {
	return &HealthStatus{
		Contract:  "relational",
		Backend:   "memory",
		State:     HealthHealthy,
		LatencyMS: 0,
	}, nil
}

// --- Sorted Set Store ---

type memorySortedSet struct {
	mu   sync.RWMutex
	sets map[string]map[string]float64
}

func (m *memorySortedSet) Add(_ context.Context, set string, member string, score float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sets[set] == nil {
		m.sets[set] = make(map[string]float64)
	}
	m.sets[set][member] = score
	return nil
}

func (m *memorySortedSet) Remove(_ context.Context, set string, member string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sets[set] != nil {
		delete(m.sets[set], member)
	}
	return nil
}

func (m *memorySortedSet) RangeByScore(_ context.Context, set string, min, max float64, limit *int) ([]ScoredMember, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []ScoredMember
	if members, ok := m.sets[set]; ok {
		for member, score := range members {
			if score >= min && score <= max {
				result = append(result, ScoredMember{Member: member, Score: score})
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Score < result[j].Score
	})

	if limit != nil && len(result) > *limit {
		result = result[:*limit]
	}

	return result, nil
}

func (m *memorySortedSet) Count(_ context.Context, set string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.sets[set])), nil
}

func (m *memorySortedSet) RemoveRange(_ context.Context, set string, min, max float64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var count int64
	if members, ok := m.sets[set]; ok {
		for member, score := range members {
			if score >= min && score <= max {
				delete(members, member)
				count++
			}
		}
	}
	return count, nil
}

func (m *memorySortedSet) Score(_ context.Context, set string, member string) (*float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if members, ok := m.sets[set]; ok {
		if score, ok := members[member]; ok {
			s := score
			return &s, nil
		}
	}
	return nil, nil
}

func (m *memorySortedSet) IsHealthy(_ context.Context) (*HealthStatus, error) {
	return &HealthStatus{
		Contract:  "sorted_set",
		Backend:   "memory",
		State:     HealthHealthy,
		LatencyMS: 0,
	}, nil
}

// --- Pub/Sub Store ---

type memoryPubSub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan Message]struct{}
}

func (m *memoryPubSub) Publish(_ context.Context, channel string, message []byte) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	msg := Message{
		Channel:   channel,
		Payload:   message,
		Timestamp: time.Now(),
	}

	if subs, ok := m.subscribers[channel]; ok {
		for ch := range subs {
			select {
			case ch <- msg:
			default:
				// Drop if channel is full to avoid blocking
			}
		}
	}
	return nil
}

func (m *memoryPubSub) Subscribe(_ context.Context, channels []string) (<-chan Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan Message, 100)
	for _, channel := range channels {
		if m.subscribers[channel] == nil {
			m.subscribers[channel] = make(map[chan Message]struct{})
		}
		m.subscribers[channel][ch] = struct{}{}
	}
	return ch, nil
}

func (m *memoryPubSub) Unsubscribe(_ context.Context, channels []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, channel := range channels {
		if _, ok := m.subscribers[channel]; ok {
			// Note: In a real implementation, we'd need to track which channel belongs to which subscriber
			// For simplicity, we just clear the channel map here or leave it to GC
			delete(m.subscribers, channel)
		}
	}
	return nil
}

func (m *memoryPubSub) IsHealthy(_ context.Context) (*HealthStatus, error) {
	return &HealthStatus{
		Contract:  "pubsub",
		Backend:   "memory",
		State:     HealthHealthy,
		LatencyMS: 0,
	}, nil
}

// --- Blob Store ---

type blobData struct {
	data     []byte
	metadata *Metadata
}

type memoryBlob struct {
	mu    sync.RWMutex
	blobs map[string]blobData
}

func (m *memoryBlob) Put(_ context.Context, key string, data []byte, metadata *Metadata) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blobs[key] = blobData{
		data:     append([]byte(nil), data...),
		metadata: metadata,
	}
	return nil
}

func (m *memoryBlob) Get(_ context.Context, key string) ([]byte, *Metadata, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if b, ok := m.blobs[key]; ok {
		return append([]byte(nil), b.data...), b.metadata, true, nil
	}
	return nil, nil, false, nil
}

func (m *memoryBlob) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.blobs, key)
	return nil
}

func (m *memoryBlob) List(_ context.Context, prefix *string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var keys []string
	for k := range m.blobs {
		if prefix == nil || len(k) >= len(*prefix) && k[:len(*prefix)] == *prefix {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

func (m *memoryBlob) PresignURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return "mock://blob-store/" + key, nil
}

func (m *memoryBlob) IsHealthy(_ context.Context) (*HealthStatus, error) {
	return &HealthStatus{
		Contract:  "blob",
		Backend:   "memory",
		State:     HealthHealthy,
		LatencyMS: 0,
	}, nil
}
