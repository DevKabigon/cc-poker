package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/DevKabigon/cc-poker/backend/internal/auth"
	"github.com/DevKabigon/cc-poker/backend/internal/config"
	"github.com/DevKabigon/cc-poker/backend/internal/protocol"
	"github.com/DevKabigon/cc-poker/backend/internal/session"
	"github.com/DevKabigon/cc-poker/backend/internal/store"
	"github.com/DevKabigon/cc-poker/backend/internal/table"
	"github.com/gorilla/websocket"
)

const wsEventVersion = 1

type guestSessionRequest struct {
	Nickname string `json:"nickname"`
}

type guestSessionResponse struct {
	PlayerID  string `json:"player_id"`
	Nickname  string `json:"nickname"`
	UserType  string `json:"user_type"`
	ExpiresAt string `json:"expires_at"`
}

type authExchangeRequest struct {
	AccessToken string `json:"access_token"`
	Nickname    string `json:"nickname"`
}

type authExchangeResponse struct {
	PlayerID      string `json:"player_id"`
	Nickname      string `json:"nickname"`
	UserType      string `json:"user_type"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	ExpiresAt     string `json:"expires_at"`
}

type nicknameCheckRequest struct {
	Nickname string `json:"nickname"`
}

type nicknameCheckResponse struct {
	Available bool `json:"available"`
}

type tableBuyInRequest struct {
	TableID string `json:"table_id"`
	Amount  int64  `json:"amount"`
}

type tableBuyInResponse struct {
	BuyInID      int64  `json:"buy_in_id"`
	PlayerID     string `json:"player_id"`
	TableID      string `json:"table_id"`
	RoomID       string `json:"room_id"`
	Amount       int64  `json:"amount"`
	BalanceAfter int64  `json:"balance_after"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
}

type healthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Timestamp string `json:"timestamp"`
}

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

func (a *App) registerRoutes() {
	a.mux.HandleFunc("/health", a.handleHealth)
	a.mux.HandleFunc("/v1/session/guest", a.handleGuestSession)
	a.mux.HandleFunc("/v1/auth/nickname/check", a.handleNicknameCheck)
	a.mux.HandleFunc("/v1/auth/exchange", a.handleAuthExchange)
	a.mux.HandleFunc("/v1/auth/logout", a.handleAuthLogout)
	a.mux.HandleFunc("/v1/tables/buy-in", a.handleTableBuyIn)
	a.mux.HandleFunc("/ws", a.handleWebSocket)
}

// handleHealth는 서버 생존 상태를 확인하기 위한 엔드포인트다.
func (a *App) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:    "ok",
		Service:   "cc-poker-backend",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// handleNicknameCheck은 닉네임 사용 가능 여부를 검사한다.
func (a *App) handleNicknameCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req nicknameCheckRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}
	}

	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" {
		http.Error(w, "nickname is required", http.StatusBadRequest)
		return
	}

	if a.sessions.IsNicknameTaken(nickname) {
		writeJSON(w, http.StatusOK, nicknameCheckResponse{Available: false})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.PostgresTimeout)
	defer cancel()

	isTaken, err := a.eventStore.IsNicknameTaken(ctx, nickname)
	if err != nil {
		a.logger.Printf("failed to check nickname availability: nickname=%s err=%v", nickname, err)
		http.Error(w, "failed to check nickname", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, nicknameCheckResponse{Available: !isTaken})
}

// handleGuestSession은 게스트 세션을 발급하고 쿠키를 설정한다.
func (a *App) handleGuestSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req guestSessionRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}
	}
	// 게스트 입장은 닉네임을 반드시 입력하도록 강제한다.
	if strings.TrimSpace(req.Nickname) == "" {
		http.Error(w, "nickname is required", http.StatusBadRequest)
		return
	}
	a.revokeSessionFromCookie(r)

	created, err := a.sessions.CreateGuest(req.Nickname, a.cfg.SessionTTL)
	if err != nil {
		if errors.Is(err, session.ErrNicknameTaken) {
			http.Error(w, "nickname is already taken", http.StatusConflict)
			return
		}
		a.logger.Printf("failed to create guest session: %v", err)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	if err := a.persistSession(created); err != nil {
		a.sessions.Delete(created.SessionID)
		statusCode, message := nicknameErrorToHTTP(err)
		http.Error(w, message, statusCode)
		return
	}
	a.ensureWallet(created.PlayerID, a.cfg.GuestWalletInitial)
	a.setSessionCookie(w, created)
	writeJSON(w, http.StatusCreated, guestSessionResponse{
		PlayerID:  created.PlayerID,
		Nickname:  created.Nickname,
		UserType:  created.UserType,
		ExpiresAt: created.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

// handleAuthExchange는 Supabase access token을 서버 세션 쿠키로 교환한다.
func (a *App) handleAuthExchange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req authExchangeRequest
	if r.Body == nil {
		http.Error(w, "request body is required", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.SupabaseTimeout)
	defer cancel()

	authUser, err := a.authVerifier.VerifyAccessToken(ctx, req.AccessToken)
	if err != nil {
		statusCode, message := authErrorToHTTP(err)
		http.Error(w, message, statusCode)
		return
	}
	a.revokeSessionFromCookie(r)

	playerID := authUserToPlayerID(authUser.UserID)
	nickname := resolveAuthNickname(req.Nickname, authUser)
	created, err := a.sessions.Create(playerID, nickname, a.cfg.SessionTTL)
	if err != nil {
		if errors.Is(err, session.ErrNicknameTaken) {
			http.Error(w, "nickname is already taken", http.StatusConflict)
			return
		}
		a.logger.Printf("failed to create auth session: %v", err)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}
	created.UserType = "auth"
	created.Email = strings.TrimSpace(authUser.Email)
	created.EmailVerified = authUser.EmailVerified

	if err := a.persistSession(created); err != nil {
		a.sessions.Delete(created.SessionID)
		statusCode, message := nicknameErrorToHTTP(err)
		http.Error(w, message, statusCode)
		return
	}
	a.ensureWallet(created.PlayerID, a.cfg.AuthWalletInitial)
	a.setSessionCookie(w, created)

	writeJSON(w, http.StatusCreated, authExchangeResponse{
		PlayerID:      created.PlayerID,
		Nickname:      created.Nickname,
		UserType:      created.UserType,
		Email:         authUser.Email,
		EmailVerified: authUser.EmailVerified,
		ExpiresAt:     created.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

// handleAuthLogout은 세션 쿠키를 만료시키고 메모리 세션을 제거한다.
func (a *App) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie(a.cfg.SessionCookieName)
	if err == nil && strings.TrimSpace(cookie.Value) != "" {
		a.sessions.Delete(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     a.cfg.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
	w.WriteHeader(http.StatusNoContent)
}

// handleTableBuyIn은 쿠키 인증된 플레이어의 테이블 바이인을 생성한다.
func (a *App) handleTableBuyIn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	playerSession, ok := a.authFromCookie(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req tableBuyInRequest
	if r.Body == nil {
		http.Error(w, "request body is required", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	tableID := a.resolveTableID(req.TableID)
	if req.Amount <= 0 {
		http.Error(w, "buy-in amount must be positive", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.PostgresTimeout)
	defer cancel()

	receipt, err := a.eventStore.CreateBuyIn(ctx, playerSession.PlayerID, tableID, req.Amount)
	if err != nil {
		statusCode, message := buyInErrorToHTTP(err)
		http.Error(w, message, statusCode)
		return
	}

	writeJSON(w, http.StatusCreated, tableBuyInResponse{
		BuyInID:      receipt.BuyInID,
		PlayerID:     receipt.PlayerID,
		TableID:      receipt.TableID,
		RoomID:       receipt.RoomID,
		Amount:       receipt.Amount,
		BalanceAfter: receipt.BalanceAfter,
		Status:       receipt.Status,
		CreatedAt:    receipt.CreatedAt.UTC().Format(time.RFC3339),
	})
}

// handleWebSocket은 세션 쿠키 인증 후 WS 연결을 수립한다.
func (a *App) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	playerSession, ok := a.authFromCookie(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		a.logger.Printf("failed to upgrade websocket: %v", err)
		return
	}

	client := newWSClient(conn, playerSession, a.cfg.DefaultTableID)
	a.hub.add(client)
	defer a.closeClient(client)

	snapshot, seq := a.tables.Snapshot(client.CurrentTableID())
	initialEnvelope := a.newSnapshotEnvelope(snapshot, seq)
	if err := a.writeEnvelope(client, initialEnvelope); err != nil {
		a.logger.Printf("failed to write initial snapshot: %v", err)
		return
	}

	for {
		var envelope protocol.ClientEnvelope
		if err := conn.ReadJSON(&envelope); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				a.logger.Printf("unexpected websocket close for player=%s: %v", playerSession.PlayerID, err)
			}
			return
		}

		if replayed, err := client.ReplayIdempotent(envelope.RequestID, envelope.EventType); err != nil {
			a.logger.Printf("failed to replay idempotent response: %v", err)
			return
		} else if replayed {
			continue
		}

		responses, err := a.dispatchClientEvent(client, envelope)
		if err != nil {
			a.logger.Printf("failed to handle websocket event=%s player=%s err=%v", envelope.EventType, playerSession.PlayerID, err)
			return
		}

		client.StoreIdempotent(envelope.RequestID, envelope.EventType, responses)
	}
}

func (a *App) dispatchClientEvent(client *wsClient, envelope protocol.ClientEnvelope) ([]protocol.ServerEnvelope, error) {
	switch envelope.EventType {
	case "join_table":
		return a.handleJoinTableEvent(client, envelope.Payload)
	case "leave_table":
		return a.handleLeaveTableEvent(client, envelope.Payload)
	default:
		return a.sendErrorNotice(client, "UNSUPPORTED_EVENT", "unsupported event type")
	}
}

func (a *App) closeClient(client *wsClient) {
	if client.IsSeated() {
		tableID := client.CurrentTableID()
		snapshot, seq, err := a.tables.Leave(tableID, client.PlayerID())
		if err != nil && !errors.Is(err, table.ErrPlayerNotFound) {
			a.logger.Printf("failed to leave table on disconnect: player=%s table=%s err=%v", client.PlayerID(), tableID, err)
		}

		if err == nil {
			a.broadcastSnapshot(tableID, snapshot, seq)
			a.persistSnapshot(snapshot, seq)
			a.persistTableEvent("disconnect_leave", client.PlayerID(), snapshot, seq)
		}
	}

	a.hub.remove(client)
	_ = client.Close()
}

func (a *App) handleJoinTableEvent(client *wsClient, payload json.RawMessage) ([]protocol.ServerEnvelope, error) {
	var joinReq protocol.JoinTablePayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &joinReq); err != nil {
			return a.sendErrorNotice(client, "INVALID_PAYLOAD", "join_table payload is invalid")
		}
	}

	tableID := a.resolveTableID(joinReq.TableID)
	currentTableID, seated := client.TableState()

	// 같은 테이블 재입장 요청은 기존 좌석 상태를 그대로 재전송해 idempotent하게 처리한다.
	if seated && currentTableID == tableID {
		snapshot, seq := a.tables.Snapshot(tableID)
		envelope := a.newSnapshotEnvelope(snapshot, seq)
		if err := a.writeEnvelope(client, envelope); err != nil {
			return nil, err
		}
		return []protocol.ServerEnvelope{envelope}, nil
	}

	if err := a.prevalidateJoinSeat(tableID, joinReq.SeatIndex); err != nil {
		code, message := tableErrorToNotice(err)
		return a.sendErrorNotice(client, code, message)
	}

	stack, err := a.consumePendingBuyIn(client.PlayerID(), tableID)
	if err != nil {
		code, message := buyInErrorToNotice(err)
		return a.sendErrorNotice(client, code, message)
	}

	// 다른 테이블에 이미 앉아 있다면 기존 테이블에서 먼저 퇴장 처리한다.
	if seated && currentTableID != tableID {
		previousSnapshot, previousSeq, leaveErr := a.tables.Leave(currentTableID, client.PlayerID())
		if leaveErr != nil && !errors.Is(leaveErr, table.ErrPlayerNotFound) {
			return nil, leaveErr
		}
		client.SetTableState(currentTableID, false)
		if leaveErr == nil {
			a.broadcastSnapshot(currentTableID, previousSnapshot, previousSeq)
			a.persistSnapshot(previousSnapshot, previousSeq)
			a.persistTableEvent("switch_leave", client.PlayerID(), previousSnapshot, previousSeq)
		}
	}

	snapshot, seq, err := a.tables.Join(tableID, client.PlayerID(), client.Nickname(), stack, joinReq.SeatIndex)
	if err != nil {
		code, message := tableErrorToNotice(err)
		return a.sendErrorNotice(client, code, message)
	}

	client.SetTableState(tableID, true)
	envelope := a.broadcastSnapshot(tableID, snapshot, seq)
	a.persistSnapshot(snapshot, seq)
	a.persistTableEvent("join_table", client.PlayerID(), snapshot, seq)
	return []protocol.ServerEnvelope{envelope}, nil
}

func (a *App) handleLeaveTableEvent(client *wsClient, payload json.RawMessage) ([]protocol.ServerEnvelope, error) {
	var leaveReq protocol.LeaveTablePayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &leaveReq); err != nil {
			return a.sendErrorNotice(client, "INVALID_PAYLOAD", "leave_table payload is invalid")
		}
	}

	tableID := client.CurrentTableID()
	if strings.TrimSpace(leaveReq.TableID) != "" {
		tableID = a.resolveTableID(leaveReq.TableID)
	}

	snapshot, seq, err := a.tables.Leave(tableID, client.PlayerID())
	if err != nil {
		code, message := tableErrorToNotice(err)
		return a.sendErrorNotice(client, code, message)
	}

	client.SetTableState(tableID, false)
	envelope := a.broadcastSnapshot(tableID, snapshot, seq)
	a.persistSnapshot(snapshot, seq)
	a.persistTableEvent("leave_table", client.PlayerID(), snapshot, seq)
	return []protocol.ServerEnvelope{envelope}, nil
}

func (a *App) broadcastSnapshot(tableID string, snapshot table.Snapshot, seq uint64) protocol.ServerEnvelope {
	envelope := a.newSnapshotEnvelope(snapshot, seq)
	a.broadcastEnvelope(tableID, envelope)
	return envelope
}

func (a *App) broadcastEnvelope(tableID string, envelope protocol.ServerEnvelope) {
	for _, client := range a.hub.clientsForTable(tableID) {
		if err := a.writeEnvelope(client, envelope); err != nil {
			a.logger.Printf("failed to broadcast envelope: player=%s table=%s err=%v", client.PlayerID(), tableID, err)
			a.hub.remove(client)
			_ = client.Close()
		}
	}
}

func (a *App) sendErrorNotice(client *wsClient, code, message string) ([]protocol.ServerEnvelope, error) {
	envelope := a.newErrorNoticeEnvelope(code, message)
	if err := a.writeEnvelope(client, envelope); err != nil {
		return nil, err
	}
	return []protocol.ServerEnvelope{envelope}, nil
}

func (a *App) newSnapshotEnvelope(snapshot table.Snapshot, seq uint64) protocol.ServerEnvelope {
	return protocol.ServerEnvelope{
		EventVersion: wsEventVersion,
		EventType:    "table_snapshot",
		TableID:      snapshot.TableID,
		Seq:          seq,
		SentAt:       time.Now().UTC().Format(time.RFC3339),
		Payload:      snapshotPayload(snapshot),
	}
}

func (a *App) newErrorNoticeEnvelope(code, message string) protocol.ServerEnvelope {
	return protocol.ServerEnvelope{
		EventVersion: wsEventVersion,
		EventType:    "error_notice",
		SentAt:       time.Now().UTC().Format(time.RFC3339),
		Payload: protocol.ErrorNoticePayload{
			Code:    code,
			Message: message,
		},
	}
}

func (a *App) writeEnvelope(client *wsClient, envelope protocol.ServerEnvelope) error {
	return client.WriteEvent(envelope)
}

func (a *App) restoreSnapshot(tableID string) {
	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.SnapshotTimeout)
	defer cancel()

	loadedSnapshot, seq, found, err := a.snapshotStore.Load(ctx, tableID)
	if err != nil {
		a.logger.Printf("failed to load table snapshot: table=%s err=%v", tableID, err)
		return
	}
	if !found {
		return
	}

	restoreTableID := tableID
	if strings.TrimSpace(loadedSnapshot.TableID) != "" {
		restoreTableID = loadedSnapshot.TableID
	}

	if err := a.tables.Restore(restoreTableID, loadedSnapshot, seq); err != nil {
		a.logger.Printf("failed to restore table snapshot: table=%s err=%v", restoreTableID, err)
		return
	}

	a.logger.Printf("restored table snapshot: table=%s players=%d seq=%d", restoreTableID, loadedSnapshot.ActivePlayers, seq)
}

func (a *App) persistSnapshot(snapshot table.Snapshot, seq uint64) {
	if !a.cfg.SnapshotEnabled {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.SnapshotTimeout)
	defer cancel()

	if err := a.snapshotStore.Save(ctx, snapshot, seq); err != nil {
		a.logger.Printf("failed to persist table snapshot: table=%s seq=%d err=%v", snapshot.TableID, seq, err)
	}
}

func (a *App) persistSession(playerSession session.PlayerSession) error {
	if !a.cfg.PostgresEnabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.PostgresTimeout)
	defer cancel()

	if err := a.eventStore.SaveSession(ctx, playerSession); err != nil {
		a.logger.Printf("failed to persist session: player=%s session=%s err=%v", playerSession.PlayerID, playerSession.SessionID, err)
		return err
	}
	return nil
}

func (a *App) ensureWallet(playerID string, initialBalance int64) {
	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.PostgresTimeout)
	defer cancel()

	if err := a.eventStore.EnsureWallet(ctx, playerID, initialBalance); err != nil {
		a.logger.Printf("failed to ensure wallet: player=%s err=%v", playerID, err)
	}
}

func (a *App) consumePendingBuyIn(playerID, tableID string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.PostgresTimeout)
	defer cancel()

	receipt, err := a.eventStore.ConsumePendingBuyIn(ctx, playerID, tableID)
	if err != nil {
		return 0, err
	}
	return receipt.Amount, nil
}

func (a *App) prevalidateJoinSeat(tableID string, seatIndex *int) error {
	if seatIndex == nil {
		return nil
	}
	if *seatIndex < 0 || *seatIndex >= table.MaxPlayers {
		return table.ErrSeatInvalid
	}

	// 바이인 소비 전에 좌석 충돌을 먼저 확인해 불필요한 차감을 줄인다.
	snapshot, _ := a.tables.Snapshot(tableID)
	for _, player := range snapshot.Players {
		if player.SeatIndex == *seatIndex {
			return table.ErrSeatTaken
		}
	}
	return nil
}

func (a *App) persistTableEvent(eventType, playerID string, snapshot table.Snapshot, seq uint64) {
	if !a.cfg.PostgresEnabled {
		return
	}

	payload, err := json.Marshal(snapshotPayload(snapshot))
	if err != nil {
		a.logger.Printf("failed to marshal table event payload: table=%s seq=%d err=%v", snapshot.TableID, seq, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.PostgresTimeout)
	defer cancel()

	if err := a.eventStore.SaveTableEvent(ctx, store.TableEvent{
		TableID:   snapshot.TableID,
		Seq:       seq,
		EventType: eventType,
		PlayerID:  playerID,
		Payload:   payload,
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		a.logger.Printf("failed to persist table event: table=%s seq=%d event=%s err=%v", snapshot.TableID, seq, eventType, err)
	}
}

func snapshotPayload(snapshot table.Snapshot) protocol.TableSnapshotPayload {
	players := make([]protocol.SnapshotPlayer, 0, len(snapshot.Players))
	for _, player := range snapshot.Players {
		players = append(players, protocol.SnapshotPlayer{
			PlayerID:  player.PlayerID,
			Nickname:  player.Nickname,
			SeatIndex: player.SeatIndex,
			Stack:     player.Stack,
		})
	}

	tableState := "waiting"
	if snapshot.CanStart {
		tableState = "ready"
	}

	return protocol.TableSnapshotPayload{
		TableState:        tableState,
		MaxPlayers:        snapshot.MaxPlayers,
		MinPlayersToStart: snapshot.MinPlayersToStart,
		ActivePlayers:     snapshot.ActivePlayers,
		CanStart:          snapshot.CanStart,
		Players:           players,
	}
}

func (a *App) authFromCookie(r *http.Request) (session.PlayerSession, bool) {
	cookie, err := r.Cookie(a.cfg.SessionCookieName)
	if err != nil || cookie.Value == "" {
		return session.PlayerSession{}, false
	}

	return a.sessions.FindValid(cookie.Value)
}

// revokeSessionFromCookie는 같은 브라우저에서 새 세션 발급 전에 기존 세션/WS를 정리한다.
func (a *App) revokeSessionFromCookie(r *http.Request) {
	cookie, err := r.Cookie(a.cfg.SessionCookieName)
	if err != nil {
		return
	}

	sessionID := strings.TrimSpace(cookie.Value)
	if sessionID == "" {
		return
	}

	playerSession, exists := a.sessions.FindValid(sessionID)
	a.sessions.Delete(sessionID)
	if !exists {
		return
	}

	// 동일 플레이어의 기존 WS 연결은 새 세션 발급 시 즉시 종료한다.
	for _, client := range a.hub.clientsForPlayer(playerSession.PlayerID) {
		a.closeClient(client)
	}
}

func (a *App) setSessionCookie(w http.ResponseWriter, playerSession session.PlayerSession) {
	ttl := int(time.Until(playerSession.ExpiresAt).Seconds())
	if ttl < 0 {
		ttl = 0
	}

	http.SetCookie(w, &http.Cookie{
		Name:     a.cfg.SessionCookieName,
		Value:    playerSession.SessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   ttl,
		Expires:  playerSession.ExpiresAt,
	})
}

func (a *App) checkOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}

	if len(a.cfg.AllowedOrigins) == 0 {
		return true
	}

	_, ok := a.cfg.AllowedOrigins[origin]
	return ok
}

func (a *App) resolveTableID(requestTableID string) string {
	trimmed := strings.TrimSpace(requestTableID)
	if trimmed == "" {
		return a.cfg.DefaultTableID
	}
	return trimmed
}

func tableErrorToNotice(err error) (string, string) {
	switch {
	case errors.Is(err, table.ErrSeatInvalid):
		return "INVALID_SEAT", "seat index is invalid"
	case errors.Is(err, table.ErrSeatTaken):
		return "SEAT_TAKEN", "seat is already taken"
	case errors.Is(err, table.ErrTableFull):
		return "TABLE_FULL", "table is full"
	case errors.Is(err, table.ErrPlayerNotFound):
		return "PLAYER_NOT_FOUND", "player is not in table"
	default:
		return "INTERNAL_ERROR", fmt.Sprintf("internal error: %v", err)
	}
}

func buyInErrorToNotice(err error) (string, string) {
	switch {
	case errors.Is(err, store.ErrPendingBuyInNotFound):
		return "BUY_IN_REQUIRED", "buy-in is required before joining table"
	case errors.Is(err, store.ErrInvalidBuyInAmount):
		return "INVALID_BUY_IN", "buy-in amount is invalid"
	case errors.Is(err, store.ErrInsufficientBalance):
		return "INSUFFICIENT_BALANCE", "wallet balance is insufficient"
	case errors.Is(err, store.ErrTableNotFound):
		return "TABLE_NOT_FOUND", "table not found"
	default:
		return "INTERNAL_ERROR", fmt.Sprintf("internal error: %v", err)
	}
}

func buyInErrorToHTTP(err error) (int, string) {
	switch {
	case errors.Is(err, store.ErrTableNotFound):
		return http.StatusNotFound, "table not found"
	case errors.Is(err, store.ErrInvalidBuyInAmount):
		return http.StatusBadRequest, "buy-in amount is out of allowed range"
	case errors.Is(err, store.ErrInsufficientBalance):
		return http.StatusConflict, "insufficient wallet balance"
	default:
		return http.StatusInternalServerError, "failed to create buy-in"
	}
}

func authErrorToHTTP(err error) (int, string) {
	switch {
	case errors.Is(err, auth.ErrAuthDisabled):
		return http.StatusServiceUnavailable, "auth exchange is disabled"
	case errors.Is(err, auth.ErrInvalidAccessToken):
		return http.StatusUnauthorized, "invalid access token"
	case errors.Is(err, auth.ErrEmailNotVerified):
		return http.StatusForbidden, "email verification is required"
	default:
		return http.StatusBadGateway, "failed to verify external auth token"
	}
}

func nicknameErrorToHTTP(err error) (int, string) {
	switch {
	case errors.Is(err, session.ErrNicknameTaken), errors.Is(err, store.ErrNicknameTaken):
		return http.StatusConflict, "nickname is already taken"
	default:
		return http.StatusInternalServerError, "failed to create session"
	}
}

func authUserToPlayerID(authUserID string) string {
	trimmed := strings.TrimSpace(authUserID)
	normalized := strings.ReplaceAll(trimmed, "-", "")
	if normalized == "" {
		return "usr_unknown"
	}
	return "usr_" + normalized
}

func resolveAuthNickname(requestNickname string, authUser auth.User) string {
	candidates := []string{
		requestNickname,
		authUser.Nickname,
		emailLocalPart(authUser.Email),
		fallbackAuthNickname(authUser.UserID),
	}
	for _, candidate := range candidates {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			continue
		}

		runes := []rune(trimmed)
		if len(runes) > 20 {
			return string(runes[:20])
		}
		return trimmed
	}
	return "User-000001"
}

func emailLocalPart(email string) string {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return ""
	}

	atIndex := strings.Index(trimmed, "@")
	if atIndex <= 0 {
		return ""
	}
	return trimmed[:atIndex]
}

func fallbackAuthNickname(userID string) string {
	trimmed := strings.TrimSpace(userID)
	trimmed = strings.ReplaceAll(trimmed, "-", "")
	if len(trimmed) >= 6 {
		return "User-" + trimmed[:6]
	}
	if trimmed != "" {
		return "User-" + trimmed
	}
	return ""
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, "failed to write json response", http.StatusInternalServerError)
	}
}
