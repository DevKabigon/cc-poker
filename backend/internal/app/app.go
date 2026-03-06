package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/DevKabigon/cc-poker/backend/internal/config"
	"github.com/DevKabigon/cc-poker/backend/internal/protocol"
	"github.com/DevKabigon/cc-poker/backend/internal/session"
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
	ExpiresAt string `json:"expires_at"`
}

type healthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Timestamp string `json:"timestamp"`
}

// App은 HTTP/WS 엔드포인트와 도메인 서비스를 조합한 애플리케이션 루트다.
type App struct {
	cfg      config.Config
	logger   *log.Logger
	sessions *session.Store
	tables   *table.Manager
	hub      *wsHub
	mux      *http.ServeMux
	upgrader websocket.Upgrader
}

// New는 앱 인스턴스를 초기화하고 라우트를 구성한다.
func New(cfg config.Config, logger *log.Logger) *App {
	if logger == nil {
		logger = log.New(os.Stdout, "", log.LstdFlags|log.LUTC)
	}

	app := &App{
		cfg:      cfg,
		logger:   logger,
		sessions: session.NewStore(nil),
		tables:   table.NewManager(cfg.DefaultTableID),
		hub:      newWSHub(),
		mux:      http.NewServeMux(),
	}

	app.upgrader = websocket.Upgrader{
		CheckOrigin: app.checkOrigin,
	}
	app.registerRoutes()

	return app
}

// Handler는 앱의 HTTP 핸들러를 반환한다.
func (a *App) Handler() http.Handler {
	return a.mux
}

func (a *App) registerRoutes() {
	a.mux.HandleFunc("/health", a.handleHealth)
	a.mux.HandleFunc("/v1/session/guest", a.handleGuestSession)
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

	created, err := a.sessions.CreateGuest(req.Nickname, a.cfg.SessionTTL)
	if err != nil {
		a.logger.Printf("failed to create guest session: %v", err)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	a.setSessionCookie(w, created)
	writeJSON(w, http.StatusCreated, guestSessionResponse{
		PlayerID:  created.PlayerID,
		Nickname:  created.Nickname,
		ExpiresAt: created.ExpiresAt.UTC().Format(time.RFC3339),
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
	if err := a.writeSnapshot(client, snapshot, seq); err != nil {
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

		switch envelope.EventType {
		case "join_table":
			if err := a.handleJoinTableEvent(client, envelope.Payload); err != nil {
				a.logger.Printf("join_table handling error: %v", err)
				return
			}
		case "leave_table":
			if err := a.handleLeaveTableEvent(client, envelope.Payload); err != nil {
				a.logger.Printf("leave_table handling error: %v", err)
				return
			}
		default:
			if err := a.writeErrorNotice(client, "UNSUPPORTED_EVENT", "unsupported event type"); err != nil {
				a.logger.Printf("failed to send error_notice: %v", err)
				return
			}
		}
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
		}
	}

	a.hub.remove(client)
	_ = client.Close()
}

func (a *App) handleJoinTableEvent(client *wsClient, payload json.RawMessage) error {
	var joinReq protocol.JoinTablePayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &joinReq); err != nil {
			return a.writeErrorNotice(client, "INVALID_PAYLOAD", "join_table payload is invalid")
		}
	}

	tableID := a.resolveTableID(joinReq.TableID)
	currentTableID, seated := client.TableState()

	// 다른 테이블에 이미 앉아 있다면 기존 테이블에서 먼저 퇴장 처리한다.
	if seated && currentTableID != tableID {
		previousSnapshot, previousSeq, leaveErr := a.tables.Leave(currentTableID, client.PlayerID())
		if leaveErr != nil && !errors.Is(leaveErr, table.ErrPlayerNotFound) {
			return leaveErr
		}
		client.SetTableState(currentTableID, false)
		if leaveErr == nil {
			a.broadcastSnapshot(currentTableID, previousSnapshot, previousSeq)
		}
	}

	snapshot, seq, err := a.tables.Join(tableID, client.PlayerID(), client.Nickname(), joinReq.SeatIndex)
	if err != nil {
		code, message := tableErrorToNotice(err)
		return a.writeErrorNotice(client, code, message)
	}

	client.SetTableState(tableID, true)
	a.broadcastSnapshot(tableID, snapshot, seq)
	return nil
}

func (a *App) handleLeaveTableEvent(client *wsClient, payload json.RawMessage) error {
	var leaveReq protocol.LeaveTablePayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &leaveReq); err != nil {
			return a.writeErrorNotice(client, "INVALID_PAYLOAD", "leave_table payload is invalid")
		}
	}

	tableID := client.CurrentTableID()
	if strings.TrimSpace(leaveReq.TableID) != "" {
		tableID = a.resolveTableID(leaveReq.TableID)
	}

	snapshot, seq, err := a.tables.Leave(tableID, client.PlayerID())
	if err != nil {
		code, message := tableErrorToNotice(err)
		return a.writeErrorNotice(client, code, message)
	}

	client.SetTableState(tableID, false)
	a.broadcastSnapshot(tableID, snapshot, seq)
	return nil
}

func (a *App) broadcastSnapshot(tableID string, snapshot table.Snapshot, seq uint64) {
	for _, client := range a.hub.clientsForTable(tableID) {
		if err := a.writeSnapshot(client, snapshot, seq); err != nil {
			a.logger.Printf("failed to broadcast snapshot: player=%s table=%s err=%v", client.PlayerID(), tableID, err)
			a.hub.remove(client)
			_ = client.Close()
		}
	}
}

func (a *App) writeSnapshot(client *wsClient, snapshot table.Snapshot, seq uint64) error {
	players := make([]protocol.SnapshotPlayer, 0, len(snapshot.Players))
	for _, player := range snapshot.Players {
		players = append(players, protocol.SnapshotPlayer{
			PlayerID:  player.PlayerID,
			Nickname:  player.Nickname,
			SeatIndex: player.SeatIndex,
		})
	}

	tableState := "waiting"
	if snapshot.CanStart {
		tableState = "ready"
	}

	payload := protocol.TableSnapshotPayload{
		TableState:        tableState,
		MaxPlayers:        snapshot.MaxPlayers,
		MinPlayersToStart: snapshot.MinPlayersToStart,
		ActivePlayers:     snapshot.ActivePlayers,
		CanStart:          snapshot.CanStart,
		Players:           players,
	}

	return a.writeWSEvent(client, "table_snapshot", snapshot.TableID, seq, payload)
}

func (a *App) writeErrorNotice(client *wsClient, code, message string) error {
	return a.writeWSEvent(client, "error_notice", "", 0, protocol.ErrorNoticePayload{
		Code:    code,
		Message: message,
	})
}

func (a *App) writeWSEvent(client *wsClient, eventType, tableID string, seq uint64, payload any) error {
	return client.WriteEvent(protocol.ServerEnvelope{
		EventVersion: wsEventVersion,
		EventType:    eventType,
		TableID:      tableID,
		Seq:          seq,
		SentAt:       time.Now().UTC().Format(time.RFC3339),
		Payload:      payload,
	})
}

func (a *App) authFromCookie(r *http.Request) (session.PlayerSession, bool) {
	cookie, err := r.Cookie(a.cfg.SessionCookieName)
	if err != nil || cookie.Value == "" {
		return session.PlayerSession{}, false
	}

	return a.sessions.FindValid(cookie.Value)
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

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, "failed to write json response", http.StatusInternalServerError)
	}
}
