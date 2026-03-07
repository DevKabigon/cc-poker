package app

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/DevKabigon/cc-poker/backend/internal/auth"
	"github.com/DevKabigon/cc-poker/backend/internal/config"
	"github.com/DevKabigon/cc-poker/backend/internal/session"
	"github.com/DevKabigon/cc-poker/backend/internal/store"
	"github.com/DevKabigon/cc-poker/backend/internal/table"
	"github.com/gorilla/websocket"
)

// App은 HTTP/WS 엔드포인트와 도메인 서비스를 조합한 애플리케이션 루트다.
type App struct {
	cfg           config.Config
	logger        *log.Logger
	sessions      *session.Store
	tables        *table.Manager
	snapshotStore store.TableSnapshotStore
	eventStore    store.EventStore
	authVerifier  auth.Verifier
	hub           *wsHub
	mux           *http.ServeMux
	upgrader      websocket.Upgrader
}

// New는 앱 인스턴스를 초기화하고 라우트를 구성한다.
func New(cfg config.Config, logger *log.Logger) *App {
	return newWithStores(cfg, logger, nil, nil, nil)
}

func newWithSnapshotStore(cfg config.Config, logger *log.Logger, snapshotStore store.TableSnapshotStore) *App {
	return newWithStores(cfg, logger, snapshotStore, nil, nil)
}

func newWithStores(
	cfg config.Config,
	logger *log.Logger,
	snapshotStore store.TableSnapshotStore,
	eventStore store.EventStore,
	authVerifier auth.Verifier,
) *App {
	if logger == nil {
		logger = log.New(os.Stdout, "", log.LstdFlags|log.LUTC)
	}

	if snapshotStore == nil {
		snapshotStore = buildSnapshotStore(cfg, logger)
	}
	if eventStore == nil {
		eventStore = buildEventStore(cfg, logger)
	}
	if authVerifier == nil {
		authVerifier = buildAuthVerifier(cfg, logger)
	}

	app := &App{
		cfg:           cfg,
		logger:        logger,
		sessions:      session.NewStore(nil),
		tables:        table.NewManager(cfg.DefaultTableID),
		snapshotStore: snapshotStore,
		eventStore:    eventStore,
		authVerifier:  authVerifier,
		hub:           newWSHub(),
		mux:           http.NewServeMux(),
	}

	app.upgrader = websocket.Upgrader{
		CheckOrigin: app.checkOrigin,
	}

	app.seedInitialData()
	app.restoreSnapshot(cfg.DefaultTableID)
	app.registerRoutes()

	return app
}

func buildSnapshotStore(cfg config.Config, logger *log.Logger) store.TableSnapshotStore {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.SnapshotTimeout)
	defer cancel()

	snapshotStore, err := store.NewSnapshotStore(ctx, store.RedisSnapshotConfig{
		Enabled:   cfg.SnapshotEnabled,
		Addr:      cfg.RedisAddr,
		Password:  cfg.RedisPassword,
		DB:        cfg.RedisDB,
		KeyPrefix: cfg.RedisKeyPrefix,
	})
	if err != nil {
		logger.Printf("snapshot store disabled: %v", err)
		return store.NewNoopSnapshotStore()
	}
	return snapshotStore
}

func buildEventStore(cfg config.Config, logger *log.Logger) store.EventStore {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.PostgresTimeout)
	defer cancel()

	eventStore, err := store.NewEventStore(ctx, store.PostgresEventStoreConfig{
		Enabled:  cfg.PostgresEnabled,
		DSN:      cfg.PostgresDSN,
		MaxConns: cfg.PostgresMaxConns,
	})
	if err != nil {
		logger.Printf("event store disabled: %v", err)
		return store.NewNoopEventStore()
	}
	return eventStore
}

func buildAuthVerifier(cfg config.Config, logger *log.Logger) auth.Verifier {
	enabled := cfg.SupabaseEnabled
	hasURL := strings.TrimSpace(cfg.SupabaseURL) != ""
	hasKey := strings.TrimSpace(cfg.SupabaseAnonKey) != ""

	logger.Printf(
		"supabase auth config: enabled=%t url_set=%t anon_key_set=%t timeout_ms=%d",
		enabled,
		hasURL,
		hasKey,
		cfg.SupabaseTimeout.Milliseconds(),
	)

	if !enabled {
		logger.Printf("supabase auth verifier disabled: CC_POKER_SUPABASE_ENABLED=false")
		return auth.NewSupabaseVerifier(auth.SupabaseConfig{Enabled: false})
	}
	if !hasURL || !hasKey {
		logger.Printf("supabase auth verifier disabled: missing URL or anon key")
		return auth.NewSupabaseVerifier(auth.SupabaseConfig{Enabled: false})
	}

	return auth.NewSupabaseVerifier(auth.SupabaseConfig{
		Enabled: true,
		URL:     cfg.SupabaseURL,
		AnonKey: cfg.SupabaseAnonKey,
		Timeout: cfg.SupabaseTimeout,
	})
}

func (a *App) seedInitialData() {
	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.PostgresTimeout)
	defer cancel()

	if err := a.eventStore.SeedRoomsAndTables(ctx); err != nil {
		a.logger.Printf("failed to seed rooms/tables: %v", err)
	}
}

// Handler는 앱의 HTTP 핸들러를 반환한다.
func (a *App) Handler() http.Handler {
	return a.mux
}
