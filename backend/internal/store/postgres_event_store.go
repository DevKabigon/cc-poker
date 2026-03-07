package store

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/DevKabigon/cc-poker/backend/internal/session"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

type seedRoom struct {
	ID         string
	Name       string
	SmallBlind int64
	BigBlind   int64
	MinBuyIn   int64
	MaxBuyIn   int64
	MaxPlayers int
}

var defaultRooms = []seedRoom{
	{
		ID:         "room_1_2",
		Name:       "$1/$2",
		SmallBlind: 1,
		BigBlind:   2,
		MinBuyIn:   100,
		MaxBuyIn:   400,
		MaxPlayers: 9,
	},
	{
		ID:         "room_2_5",
		Name:       "$2/$5",
		SmallBlind: 2,
		BigBlind:   5,
		MinBuyIn:   250,
		MaxBuyIn:   1000,
		MaxPlayers: 9,
	},
	{
		ID:         "room_5_10",
		Name:       "$5/$10",
		SmallBlind: 5,
		BigBlind:   10,
		MinBuyIn:   500,
		MaxBuyIn:   2000,
		MaxPlayers: 9,
	},
}

// SeedRoomsAndTables는 기본 룸/테이블 메타데이터를 DB에 삽입한다.
func (s *postgresEventStore) SeedRoomsAndTables(ctx context.Context) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, room := range defaultRooms {
		if _, err := tx.Exec(ctx, `
			INSERT INTO rooms (id, name, small_blind, big_blind, min_buy_in, max_buy_in, max_players)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE
			SET name = EXCLUDED.name,
				small_blind = EXCLUDED.small_blind,
				big_blind = EXCLUDED.big_blind,
				min_buy_in = EXCLUDED.min_buy_in,
				max_buy_in = EXCLUDED.max_buy_in,
				max_players = EXCLUDED.max_players
		`, room.ID, room.Name, room.SmallBlind, room.BigBlind, room.MinBuyIn, room.MaxBuyIn, room.MaxPlayers); err != nil {
			return fmt.Errorf("failed to upsert room: %w", err)
		}

		for index := 1; index <= 10; index++ {
			tableID := fmt.Sprintf("%s_table_%d", room.ID, index)
			label := fmt.Sprintf("%s Table %d", room.Name, index)
			if _, err := tx.Exec(ctx, `
				INSERT INTO tables (id, room_id, label, max_players, is_active)
				VALUES ($1, $2, $3, $4, TRUE)
				ON CONFLICT (id) DO UPDATE
				SET room_id = EXCLUDED.room_id,
					label = EXCLUDED.label,
					max_players = EXCLUDED.max_players,
					is_active = TRUE
			`, tableID, room.ID, label, room.MaxPlayers); err != nil {
				return fmt.Errorf("failed to upsert table: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit room seed tx: %w", err)
	}
	return nil
}

// EnsureWallet는 플레이어 지갑 레코드가 없으면 기본 잔액으로 생성한다.
func (s *postgresEventStore) EnsureWallet(ctx context.Context, playerID string, initialBalance int64) error {
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO wallets (player_id, balance)
		VALUES ($1, $2)
		ON CONFLICT (player_id) DO NOTHING
	`, playerID, initialBalance); err != nil {
		return fmt.Errorf("failed to ensure wallet: %w", err)
	}
	return nil
}

// CreateBuyIn은 바이인 금액 검증/지갑 차감/바이인 생성을 원자적으로 처리한다.
func (s *postgresEventStore) CreateBuyIn(ctx context.Context, playerID, tableID string, amount int64) (BuyInReceipt, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return BuyInReceipt{}, fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var roomID string
	var minBuyIn int64
	var maxBuyIn int64
	if err := tx.QueryRow(ctx, `
		SELECT r.id, r.min_buy_in, r.max_buy_in
		FROM tables t
		JOIN rooms r ON r.id = t.room_id
		WHERE t.id = $1 AND t.is_active = TRUE
	`, tableID).Scan(&roomID, &minBuyIn, &maxBuyIn); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BuyInReceipt{}, ErrTableNotFound
		}
		return BuyInReceipt{}, fmt.Errorf("failed to load room/table for buy-in: %w", err)
	}

	if amount < minBuyIn || amount > maxBuyIn {
		return BuyInReceipt{}, ErrInvalidBuyInAmount
	}

	var pending BuyInReceipt
	pendingErr := tx.QueryRow(ctx, `
		SELECT id, player_id, table_id, room_id, amount, status, created_at
		FROM buy_ins
		WHERE player_id = $1 AND table_id = $2 AND status = 'pending'
		ORDER BY id DESC
		LIMIT 1
	`, playerID, tableID).Scan(
		&pending.BuyInID,
		&pending.PlayerID,
		&pending.TableID,
		&pending.RoomID,
		&pending.Amount,
		&pending.Status,
		&pending.CreatedAt,
	)
	if pendingErr == nil {
		var currentBalance int64
		if err := tx.QueryRow(ctx, `SELECT balance FROM wallets WHERE player_id = $1`, playerID).Scan(&currentBalance); err == nil {
			pending.BalanceAfter = currentBalance
		}
		if err := tx.Commit(ctx); err != nil {
			return BuyInReceipt{}, fmt.Errorf("failed to commit pending buy-in tx: %w", err)
		}
		return pending, nil
	}
	if pendingErr != nil && !errors.Is(pendingErr, pgx.ErrNoRows) {
		return BuyInReceipt{}, fmt.Errorf("failed to query pending buy-in: %w", pendingErr)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO wallets (player_id, balance)
		VALUES ($1, $2)
		ON CONFLICT (player_id) DO NOTHING
	`, playerID, int64(0)); err != nil {
		return BuyInReceipt{}, fmt.Errorf("failed to ensure wallet in buy-in: %w", err)
	}

	var currentBalance int64
	if err := tx.QueryRow(ctx, `
		SELECT balance
		FROM wallets
		WHERE player_id = $1
		FOR UPDATE
	`, playerID).Scan(&currentBalance); err != nil {
		return BuyInReceipt{}, fmt.Errorf("failed to lock wallet: %w", err)
	}

	if currentBalance < amount {
		return BuyInReceipt{}, ErrInsufficientBalance
	}

	updatedBalance := currentBalance - amount
	if _, err := tx.Exec(ctx, `
		UPDATE wallets
		SET balance = $2, updated_at = NOW()
		WHERE player_id = $1
	`, playerID, updatedBalance); err != nil {
		return BuyInReceipt{}, fmt.Errorf("failed to update wallet balance: %w", err)
	}

	var receipt BuyInReceipt
	if err := tx.QueryRow(ctx, `
		INSERT INTO buy_ins (player_id, table_id, room_id, amount, status)
		VALUES ($1, $2, $3, $4, 'pending')
		RETURNING id, player_id, table_id, room_id, amount, status, created_at
	`, playerID, tableID, roomID, amount).Scan(
		&receipt.BuyInID,
		&receipt.PlayerID,
		&receipt.TableID,
		&receipt.RoomID,
		&receipt.Amount,
		&receipt.Status,
		&receipt.CreatedAt,
	); err != nil {
		return BuyInReceipt{}, fmt.Errorf("failed to create pending buy-in: %w", err)
	}
	receipt.BalanceAfter = updatedBalance

	if err := tx.Commit(ctx); err != nil {
		return BuyInReceipt{}, fmt.Errorf("failed to commit buy-in tx: %w", err)
	}
	return receipt, nil
}

// ConsumePendingBuyIn은 입장 직전에 pending 바이인을 consumed 상태로 전환한다.
func (s *postgresEventStore) ConsumePendingBuyIn(ctx context.Context, playerID, tableID string) (BuyInReceipt, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return BuyInReceipt{}, fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var receipt BuyInReceipt
	if err := tx.QueryRow(ctx, `
		SELECT id, player_id, table_id, room_id, amount, status, created_at
		FROM buy_ins
		WHERE player_id = $1 AND table_id = $2 AND status = 'pending'
		ORDER BY id DESC
		LIMIT 1
		FOR UPDATE
	`, playerID, tableID).Scan(
		&receipt.BuyInID,
		&receipt.PlayerID,
		&receipt.TableID,
		&receipt.RoomID,
		&receipt.Amount,
		&receipt.Status,
		&receipt.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BuyInReceipt{}, ErrPendingBuyInNotFound
		}
		return BuyInReceipt{}, fmt.Errorf("failed to lock pending buy-in: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE buy_ins
		SET status = 'consumed', consumed_at = NOW()
		WHERE id = $1
	`, receipt.BuyInID); err != nil {
		return BuyInReceipt{}, fmt.Errorf("failed to mark buy-in consumed: %w", err)
	}

	var balance int64
	if err := tx.QueryRow(ctx, `
		SELECT balance
		FROM wallets
		WHERE player_id = $1
	`, playerID).Scan(&balance); err != nil {
		return BuyInReceipt{}, fmt.Errorf("failed to read balance after consume: %w", err)
	}
	receipt.BalanceAfter = balance
	receipt.Status = "consumed"

	if err := tx.Commit(ctx); err != nil {
		return BuyInReceipt{}, fmt.Errorf("failed to commit consume tx: %w", err)
	}
	return receipt, nil
}

// IsNicknameTaken은 users 테이블 기준 닉네임 중복 여부를 조회한다.
func (s *postgresEventStore) IsNicknameTaken(ctx context.Context, nickname string) (bool, error) {
	trimmed := strings.TrimSpace(nickname)
	if trimmed == "" {
		return false, nil
	}

	var exists bool
	if err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM users
			WHERE LOWER(nickname) = LOWER($1)
		)
	`, trimmed).Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check nickname duplication: %w", err)
	}
	return exists, nil
}

// SaveSession은 유저/세션 정보를 저장한다.
func (s *postgresEventStore) SaveSession(ctx context.Context, playerSession session.PlayerSession) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, nickname, user_type, email, email_verified)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5)
		ON CONFLICT (id) DO UPDATE
		SET nickname = EXCLUDED.nickname,
			user_type = EXCLUDED.user_type,
			email = COALESCE(EXCLUDED.email, users.email),
			email_verified = CASE
				WHEN EXCLUDED.email IS NULL THEN users.email_verified
				ELSE EXCLUDED.email_verified
			END
	`, playerSession.PlayerID, playerSession.Nickname, playerSession.UserType, playerSession.Email, playerSession.EmailVerified); err != nil {
		if isNicknameUniqueViolation(err) {
			return ErrNicknameTaken
		}
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

func isNicknameUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Code != "23505" {
		return false
	}
	return strings.Contains(strings.ToLower(pgErr.ConstraintName), "nickname")
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
