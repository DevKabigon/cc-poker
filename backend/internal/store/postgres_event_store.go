package store

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/DevKabigon/cc-poker/backend/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// PostgresEventStoreConfig는 Postgres 이벤트 저장소 구성값이다.
type PostgresEventStoreConfig struct {
	Enabled  bool
	DSN      string
	MaxConns int
}

// NewEventStore는 설정값에 맞는 이벤트 저장소를 생성한다.
func NewEventStore(ctx context.Context, cfg PostgresEventStoreConfig) (EventStore, error) {
	if !cfg.Enabled {
		return NewNoopEventStore(), nil
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres dsn: %w", err)
	}
	if cfg.MaxConns > 0 {
		poolConfig.MaxConns = int32(cfg.MaxConns)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	if err := applyMigrations(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to apply migrations: %w", err)
	}

	return &postgresEventStore{
		pool: pool,
	}, nil
}

type postgresEventStore struct {
	pool *pgxpool.Pool
}

// SaveSession은 유저/세션 정보를 저장한다.
func (s *postgresEventStore) SaveSession(ctx context.Context, playerSession session.PlayerSession) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, nickname)
		VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE SET nickname = EXCLUDED.nickname
	`, playerSession.PlayerID, playerSession.Nickname); err != nil {
		return fmt.Errorf("failed to upsert user: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO sessions (session_id, player_id, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (session_id) DO UPDATE
		SET player_id = EXCLUDED.player_id,
			expires_at = EXCLUDED.expires_at
	`, playerSession.SessionID, playerSession.PlayerID, playerSession.ExpiresAt.UTC()); err != nil {
		return fmt.Errorf("failed to upsert session: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit tx: %w", err)
	}
	return nil
}

// SaveTableEvent는 테이블 이벤트를 append-only로 기록한다.
func (s *postgresEventStore) SaveTableEvent(ctx context.Context, event TableEvent) error {
	payload := event.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	createdAt := event.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	if _, err := s.pool.Exec(ctx, `
		INSERT INTO table_events (table_id, seq, event_type, player_id, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, event.TableID, event.Seq, event.EventType, event.PlayerID, payload, createdAt); err != nil {
		return fmt.Errorf("failed to insert table event: %w", err)
	}
	return nil
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		version, err := parseMigrationVersion(entry.Name())
		if err != nil {
			return err
		}

		var alreadyApplied bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&alreadyApplied); err != nil {
			return err
		}
		if alreadyApplied {
			continue
		}

		content, err := migrationFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}

		if err := execStatements(ctx, tx, string(content)); err != nil {
			tx.Rollback(ctx)
			return err
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO schema_migrations (version, name, applied_at)
			VALUES ($1, $2, $3)
		`, version, entry.Name(), time.Now().UTC()); err != nil {
			tx.Rollback(ctx)
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}

	return nil
}

func execStatements(ctx context.Context, tx pgx.Tx, sqlText string) error {
	statements := strings.Split(sqlText, ";")
	for _, raw := range statements {
		stmt := strings.TrimSpace(raw)
		if stmt == "" {
			continue
		}
		if _, err := tx.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func parseMigrationVersion(fileName string) (int64, error) {
	split := strings.SplitN(fileName, "_", 2)
	if len(split) != 2 {
		return 0, fmt.Errorf("invalid migration file name: %s", fileName)
	}

	parsed, err := strconv.ParseInt(split[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid migration version in %s: %w", fileName, err)
	}
	return parsed, nil
}
