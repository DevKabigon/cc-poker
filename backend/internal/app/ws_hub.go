package app

import (
	"sync"
	"time"

	"github.com/DevKabigon/cc-poker/backend/internal/protocol"
	"github.com/DevKabigon/cc-poker/backend/internal/session"
	"github.com/gorilla/websocket"
)

const wsWriteTimeout = 5 * time.Second
const (
	maxIdempotencyEntries = 128
	idempotencyCacheTTL   = 10 * time.Minute
)

type cachedResponse struct {
	EventType string
	Envelopes []protocol.ServerEnvelope
	StoredAt  time.Time
}

// wsClient는 단일 WebSocket 연결과 플레이어 컨텍스트를 함께 관리한다.
type wsClient struct {
	conn   *websocket.Conn
	player session.PlayerSession

	writeMu sync.Mutex
	stateMu sync.RWMutex
	tableID string
	seated  bool

	requestMu    sync.Mutex
	requestOrder []string
	requestCache map[string]cachedResponse
}

func newWSClient(conn *websocket.Conn, player session.PlayerSession, initialTableID string) *wsClient {
	return &wsClient{
		conn:         conn,
		player:       player,
		tableID:      initialTableID,
		requestOrder: make([]string, 0, maxIdempotencyEntries),
		requestCache: make(map[string]cachedResponse),
	}
}

// WriteEvent는 단일 연결에 대한 WS 쓰기를 직렬화해 동시 쓰기 충돌을 막는다.
func (c *wsClient) WriteEvent(event protocol.ServerEnvelope) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	_ = c.conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout))
	return c.conn.WriteJSON(event)
}

func (c *wsClient) PlayerID() string {
	return c.player.PlayerID
}

func (c *wsClient) Nickname() string {
	return c.player.Nickname
}

func (c *wsClient) CurrentTableID() string {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.tableID
}

func (c *wsClient) IsSeated() bool {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.seated
}

func (c *wsClient) TableState() (string, bool) {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.tableID, c.seated
}

func (c *wsClient) SetTableState(tableID string, seated bool) {
	c.stateMu.Lock()
	c.tableID = tableID
	c.seated = seated
	c.stateMu.Unlock()
}

func (c *wsClient) Close() error {
	return c.conn.Close()
}

// ReplayIdempotent는 같은 request_id가 이미 처리된 경우 이전 응답을 재전송한다.
func (c *wsClient) ReplayIdempotent(requestID, eventType string) (bool, error) {
	if requestID == "" {
		return false, nil
	}

	c.requestMu.Lock()
	c.evictExpiredLocked(time.Now())

	cached, ok := c.requestCache[requestID]
	if !ok || cached.EventType != eventType {
		c.requestMu.Unlock()
		return false, nil
	}

	envelopes := append([]protocol.ServerEnvelope(nil), cached.Envelopes...)
	c.requestMu.Unlock()

	for _, envelope := range envelopes {
		if err := c.WriteEvent(envelope); err != nil {
			return true, err
		}
	}
	return true, nil
}

// StoreIdempotent는 request_id와 처리 결과를 캐시에 저장한다.
func (c *wsClient) StoreIdempotent(requestID, eventType string, responses []protocol.ServerEnvelope) {
	if requestID == "" || len(responses) == 0 {
		return
	}

	c.requestMu.Lock()
	defer c.requestMu.Unlock()

	c.evictExpiredLocked(time.Now())
	if _, exists := c.requestCache[requestID]; exists {
		return
	}

	c.requestCache[requestID] = cachedResponse{
		EventType: eventType,
		Envelopes: append([]protocol.ServerEnvelope(nil), responses...),
		StoredAt:  time.Now(),
	}
	c.requestOrder = append(c.requestOrder, requestID)

	for len(c.requestOrder) > maxIdempotencyEntries {
		oldestID := c.requestOrder[0]
		c.requestOrder = c.requestOrder[1:]
		delete(c.requestCache, oldestID)
	}
}

func (c *wsClient) evictExpiredLocked(now time.Time) {
	if len(c.requestOrder) == 0 {
		return
	}

	filtered := c.requestOrder[:0]
	for _, requestID := range c.requestOrder {
		cached, exists := c.requestCache[requestID]
		if !exists {
			continue
		}

		if now.Sub(cached.StoredAt) > idempotencyCacheTTL {
			delete(c.requestCache, requestID)
			continue
		}
		filtered = append(filtered, requestID)
	}
	c.requestOrder = filtered
}

// wsHub는 현재 접속 중인 WS 클라이언트 집합을 관리한다.
type wsHub struct {
	mu      sync.RWMutex
	clients map[*wsClient]struct{}
}

func newWSHub() *wsHub {
	return &wsHub{
		clients: make(map[*wsClient]struct{}),
	}
}

func (h *wsHub) add(client *wsClient) {
	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()
}

func (h *wsHub) remove(client *wsClient) {
	h.mu.Lock()
	delete(h.clients, client)
	h.mu.Unlock()
}

func (h *wsHub) clientsForTable(tableID string) []*wsClient {
	h.mu.RLock()
	defer h.mu.RUnlock()

	out := make([]*wsClient, 0, len(h.clients))
	for client := range h.clients {
		if client.CurrentTableID() != tableID {
			continue
		}
		out = append(out, client)
	}
	return out
}
