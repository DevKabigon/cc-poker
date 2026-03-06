package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/DevKabigon/cc-poker/backend/internal/table"
	"github.com/redis/go-redis/v9"
)

const (
	defaultRedisAddress   = "127.0.0.1:6379"
	defaultRedisKeyPrefix = "cc_poker"
)

// TableSnapshotStore는 테이블 스냅샷 저장/복구 인터페이스다.
type TableSnapshotStore interface {
	Save(ctx context.Context, snapshot table.Snapshot, seq uint64) error
	Load(ctx context.Context, tableID string) (table.Snapshot, uint64, bool, error)
}

// NewNoopSnapshotStore는 저장 동작을 수행하지 않는 기본 구현체를 반환한다.
func NewNoopSnapshotStore() TableSnapshotStore {
	return noopSnapshotStore{}
}

// RedisSnapshotConfig는 Redis 기반 스냅샷 저장소 설정값이다.
type RedisSnapshotConfig struct {
	Enabled   bool
	Addr      string
	Password  string
	DB        int
	KeyPrefix string
}

// NewSnapshotStore는 설정값에 맞는 스냅샷 저장소 구현체를 생성한다.
func NewSnapshotStore(ctx context.Context, cfg RedisSnapshotConfig) (TableSnapshotStore, error) {
	if !cfg.Enabled {
		return noopSnapshotStore{}, nil
	}

	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		addr = defaultRedisAddress
	}

	prefix := strings.TrimSpace(cfg.KeyPrefix)
	if prefix == "" {
		prefix = defaultRedisKeyPrefix
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to connect redis snapshot store: %w", err)
	}

	return &redisSnapshotStore{
		client:    client,
		keyPrefix: prefix,
	}, nil
}

type noopSnapshotStore struct{}

func (noopSnapshotStore) Save(context.Context, table.Snapshot, uint64) error {
	return nil
}

func (noopSnapshotStore) Load(context.Context, string) (table.Snapshot, uint64, bool, error) {
	return table.Snapshot{}, 0, false, nil
}

type redisSnapshotStore struct {
	client    *redis.Client
	keyPrefix string
}

type persistedSnapshot struct {
	TableID   string         `json:"table_id"`
	Seq       uint64         `json:"seq"`
	Players   []table.Player `json:"players"`
	UpdatedAt string         `json:"updated_at"`
}

// Save는 테이블 스냅샷을 Redis에 JSON으로 저장한다.
func (s *redisSnapshotStore) Save(ctx context.Context, snapshot table.Snapshot, seq uint64) error {
	payload := persistedSnapshot{
		TableID:   snapshot.TableID,
		Seq:       seq,
		Players:   snapshot.Players,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal persisted snapshot: %w", err)
	}

	if err := s.client.Set(ctx, s.tableKey(snapshot.TableID), encoded, 0).Err(); err != nil {
		return fmt.Errorf("failed to save snapshot in redis: %w", err)
	}
	return nil
}

// Load는 Redis에 저장된 스냅샷을 조회해 메모리 복구 형태로 반환한다.
func (s *redisSnapshotStore) Load(ctx context.Context, tableID string) (table.Snapshot, uint64, bool, error) {
	encoded, err := s.client.Get(ctx, s.tableKey(tableID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return table.Snapshot{}, 0, false, nil
		}
		return table.Snapshot{}, 0, false, fmt.Errorf("failed to load snapshot from redis: %w", err)
	}

	var loaded persistedSnapshot
	if err := json.Unmarshal(encoded, &loaded); err != nil {
		return table.Snapshot{}, 0, false, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	snapshot := table.Snapshot{
		TableID:           loaded.TableID,
		MaxPlayers:        table.MaxPlayers,
		MinPlayersToStart: table.MinPlayersToStart,
		ActivePlayers:     len(loaded.Players),
		CanStart:          len(loaded.Players) >= table.MinPlayersToStart,
		Players:           loaded.Players,
	}

	return snapshot, loaded.Seq, true, nil
}

func (s *redisSnapshotStore) tableKey(tableID string) string {
	return fmt.Sprintf("%s:table_snapshot:%s", s.keyPrefix, tableID)
}
