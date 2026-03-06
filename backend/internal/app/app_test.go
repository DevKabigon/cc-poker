package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DevKabigon/cc-poker/backend/internal/config"
	"github.com/DevKabigon/cc-poker/backend/internal/protocol"
	"github.com/DevKabigon/cc-poker/backend/internal/store"
	"github.com/DevKabigon/cc-poker/backend/internal/table"
	"github.com/gorilla/websocket"
)

func newTestApp() *App {
	return newWithSnapshotStore(config.Config{
		HTTPAddr:          ":0",
		SessionCookieName: "cc_poker_session",
		SessionTTL:        time.Hour,
		CookieSecure:      false,
		AllowedOrigins:    map[string]struct{}{},
		DefaultTableID:    "table-1",
		SnapshotEnabled:   false,
		SnapshotTimeout:   time.Second,
	}, log.New(io.Discard, "", 0), nil)
}

func newTestAppWithStore(snapshotStore store.TableSnapshotStore) *App {
	return newWithSnapshotStore(config.Config{
		HTTPAddr:          ":0",
		SessionCookieName: "cc_poker_session",
		SessionTTL:        time.Hour,
		CookieSecure:      false,
		AllowedOrigins:    map[string]struct{}{},
		DefaultTableID:    "table-1",
		SnapshotEnabled:   true,
		SnapshotTimeout:   time.Second,
	}, log.New(io.Discard, "", 0), snapshotStore)
}

func TestGuestSessionSetsCookie(t *testing.T) {
	app := newTestApp()

	reqBody := bytes.NewBufferString(`{"nickname":"Kabigon"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/session/guest", reqBody)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.Handler().ServeHTTP(rec, req)
	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status code: got=%d want=%d", res.StatusCode, http.StatusCreated)
	}

	cookies := res.Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected session cookie to be set")
	}

	if cookies[0].Name != "cc_poker_session" {
		t.Fatalf("unexpected cookie name: got=%s", cookies[0].Name)
	}

	if !cookies[0].HttpOnly {
		t.Fatalf("expected HttpOnly cookie")
	}
}

func TestWebSocketRejectsWithoutCookie(t *testing.T) {
	app := newTestApp()
	server := httptest.NewServer(app.Handler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatalf("expected websocket dial error without cookie")
	}

	if resp == nil {
		t.Fatalf("expected HTTP response for unauthorized websocket dial")
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected status code: got=%d want=%d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestWebSocketConnectsAndReturnsSnapshot(t *testing.T) {
	app := newTestApp()
	server := httptest.NewServer(app.Handler())
	defer server.Close()

	cookie := issueGuestCookie(t, server.URL)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	header := http.Header{}
	header.Add("Cookie", cookie.Name+"="+cookie.Value)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	var envelope protocol.ServerEnvelopeRaw
	if err := conn.ReadJSON(&envelope); err != nil {
		t.Fatalf("failed to read websocket event: %v", err)
	}

	if envelope.EventType != "table_snapshot" {
		t.Fatalf("unexpected event type: got=%s want=table_snapshot", envelope.EventType)
	}

	var payload protocol.TableSnapshotPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		t.Fatalf("failed to unmarshal snapshot payload: %v", err)
	}

	if payload.MinPlayersToStart != table.MinPlayersToStart {
		t.Fatalf("unexpected min players: got=%d want=%d", payload.MinPlayersToStart, table.MinPlayersToStart)
	}

	if payload.MaxPlayers != table.MaxPlayers {
		t.Fatalf("unexpected max players: got=%d want=%d", payload.MaxPlayers, table.MaxPlayers)
	}
}

func TestWebSocketBroadcastsSnapshotToOtherClientsOnJoin(t *testing.T) {
	app := newTestApp()
	server := httptest.NewServer(app.Handler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	cookie1 := issueGuestCookie(t, server.URL)
	conn1 := dialWSWithCookie(t, wsURL, cookie1)
	defer conn1.Close()

	snapshot := readSnapshot(t, conn1)
	if snapshot.ActivePlayers != 0 {
		t.Fatalf("unexpected active players on first connect: got=%d want=0", snapshot.ActivePlayers)
	}

	sendJoinTable(t, conn1, "table-1")
	snapshot = readSnapshot(t, conn1)
	if snapshot.ActivePlayers != 1 {
		t.Fatalf("unexpected active players after first join: got=%d want=1", snapshot.ActivePlayers)
	}

	cookie2 := issueGuestCookie(t, server.URL)
	conn2 := dialWSWithCookie(t, wsURL, cookie2)
	defer conn2.Close()

	snapshot = readSnapshot(t, conn2)
	if snapshot.ActivePlayers != 1 {
		t.Fatalf("unexpected active players on second connect: got=%d want=1", snapshot.ActivePlayers)
	}

	sendJoinTable(t, conn2, "table-1")
	snapshot2 := readSnapshot(t, conn2)
	if snapshot2.ActivePlayers != 2 {
		t.Fatalf("unexpected active players after second join(self): got=%d want=2", snapshot2.ActivePlayers)
	}

	snapshot1Broadcast := readSnapshot(t, conn1)
	if snapshot1Broadcast.ActivePlayers != 2 {
		t.Fatalf("unexpected active players after second join(broadcast): got=%d want=2", snapshot1Broadcast.ActivePlayers)
	}
}

func TestWebSocketJoinIsIdempotentByRequestID(t *testing.T) {
	app := newTestApp()
	server := httptest.NewServer(app.Handler())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	cookie := issueGuestCookie(t, server.URL)
	conn := dialWSWithCookie(t, wsURL, cookie)
	defer conn.Close()

	_ = readSnapshotEnvelope(t, conn)

	sendJoinTableWithRequestID(t, conn, "table-1", "req-join-001")
	first := readSnapshotEnvelope(t, conn)
	if first.Payload.ActivePlayers != 1 {
		t.Fatalf("unexpected active players after first join: got=%d want=1", first.Payload.ActivePlayers)
	}

	sendJoinTableWithRequestID(t, conn, "table-1", "req-join-001")
	second := readSnapshotEnvelope(t, conn)
	if second.Payload.ActivePlayers != 1 {
		t.Fatalf("unexpected active players after idempotent replay: got=%d want=1", second.Payload.ActivePlayers)
	}
	if second.Envelope.Seq != first.Envelope.Seq {
		t.Fatalf("expected duplicated request to replay same seq: first=%d second=%d", first.Envelope.Seq, second.Envelope.Seq)
	}
}

func TestAppRestoresSnapshotFromStore(t *testing.T) {
	fakeStore := &fakeSnapshotStore{
		loadSnapshot: table.Snapshot{
			TableID:           "table-1",
			MaxPlayers:        table.MaxPlayers,
			MinPlayersToStart: table.MinPlayersToStart,
			ActivePlayers:     1,
			CanStart:          false,
			Players: []table.Player{
				{PlayerID: "ply_restored", Nickname: "Restored", SeatIndex: 2},
			},
		},
		loadSeq:   7,
		loadFound: true,
	}

	app := newTestAppWithStore(fakeStore)
	snapshot, seq := app.tables.Snapshot("table-1")

	if seq != 7 {
		t.Fatalf("unexpected restored seq: got=%d want=7", seq)
	}
	if snapshot.ActivePlayers != 1 {
		t.Fatalf("unexpected restored active players: got=%d want=1", snapshot.ActivePlayers)
	}
	if len(snapshot.Players) != 1 || snapshot.Players[0].SeatIndex != 2 {
		t.Fatalf("unexpected restored players payload")
	}
}

func dialWSWithCookie(t *testing.T, wsURL string, cookie *http.Cookie) *websocket.Conn {
	t.Helper()

	header := http.Header{}
	header.Add("Cookie", cookie.Name+"="+cookie.Value)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	return conn
}

func sendJoinTable(t *testing.T, conn *websocket.Conn, tableID string) {
	sendJoinTableWithRequestID(t, conn, tableID, "")
}

func sendJoinTableWithRequestID(t *testing.T, conn *websocket.Conn, tableID, requestID string) {
	t.Helper()

	event := protocol.ClientEnvelope{
		EventType: "join_table",
		RequestID: requestID,
		Payload:   mustRawMessage(t, protocol.JoinTablePayload{TableID: tableID}),
	}

	_ = conn.SetWriteDeadline(time.Now().Add(time.Second))
	if err := conn.WriteJSON(event); err != nil {
		t.Fatalf("failed to send join_table: %v", err)
	}
}

func readSnapshot(t *testing.T, conn *websocket.Conn) protocol.TableSnapshotPayload {
	return readSnapshotEnvelope(t, conn).Payload
}

type snapshotEnvelopeResult struct {
	Envelope protocol.ServerEnvelopeRaw
	Payload  protocol.TableSnapshotPayload
}

func readSnapshotEnvelope(t *testing.T, conn *websocket.Conn) snapshotEnvelopeResult {
	t.Helper()

	var envelope protocol.ServerEnvelopeRaw
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if err := conn.ReadJSON(&envelope); err != nil {
		t.Fatalf("failed to read websocket event: %v", err)
	}

	if envelope.EventType != "table_snapshot" {
		t.Fatalf("unexpected event type: got=%s want=table_snapshot", envelope.EventType)
	}

	var snapshot protocol.TableSnapshotPayload
	if err := json.Unmarshal(envelope.Payload, &snapshot); err != nil {
		t.Fatalf("failed to decode snapshot payload: %v", err)
	}

	return snapshotEnvelopeResult{
		Envelope: envelope,
		Payload:  snapshot,
	}
}

func mustRawMessage(t *testing.T, payload any) json.RawMessage {
	t.Helper()

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	return encoded
}

func issueGuestCookie(t *testing.T, baseURL string) *http.Cookie {
	t.Helper()

	reqBody := bytes.NewBufferString(`{"nickname":"Kabigon"}`)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/session/guest", reqBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to create guest session: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status code: got=%d want=%d", res.StatusCode, http.StatusCreated)
	}

	cookies := res.Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected cookie in response")
	}

	return cookies[0]
}

type fakeSnapshotStore struct {
	loadSnapshot table.Snapshot
	loadSeq      uint64
	loadFound    bool
	loadErr      error
	saveErr      error
}

func (f *fakeSnapshotStore) Save(context.Context, table.Snapshot, uint64) error {
	return f.saveErr
}

func (f *fakeSnapshotStore) Load(context.Context, string) (table.Snapshot, uint64, bool, error) {
	return f.loadSnapshot, f.loadSeq, f.loadFound, f.loadErr
}
