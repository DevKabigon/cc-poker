package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DevKabigon/cc-poker/backend/internal/auth"
	"github.com/DevKabigon/cc-poker/backend/internal/config"
	"github.com/DevKabigon/cc-poker/backend/internal/protocol"
	"github.com/DevKabigon/cc-poker/backend/internal/store"
	"github.com/DevKabigon/cc-poker/backend/internal/table"
	"github.com/gorilla/websocket"
)

func newTestApp() *App {
	return newWithSnapshotStore(config.Config{
		HTTPAddr:           ":0",
		SessionCookieName:  "cc_poker_session",
		SessionTTL:         time.Hour,
		CookieSecure:       false,
		AllowedOrigins:     map[string]struct{}{},
		DefaultTableID:     "table-1",
		SnapshotEnabled:    false,
		SnapshotTimeout:    time.Second,
		PostgresTimeout:    time.Second,
		GuestWalletInitial: 2000,
		AuthWalletInitial:  10000,
	}, log.New(io.Discard, "", 0), nil)
}

func newTestAppWithStore(snapshotStore store.TableSnapshotStore) *App {
	return newWithSnapshotStore(config.Config{
		HTTPAddr:           ":0",
		SessionCookieName:  "cc_poker_session",
		SessionTTL:         time.Hour,
		CookieSecure:       false,
		AllowedOrigins:     map[string]struct{}{},
		DefaultTableID:     "table-1",
		SnapshotEnabled:    true,
		SnapshotTimeout:    time.Second,
		PostgresTimeout:    time.Second,
		GuestWalletInitial: 2000,
		AuthWalletInitial:  10000,
	}, log.New(io.Discard, "", 0), snapshotStore)
}

func newTestAppWithAuthVerifier(verifier auth.Verifier) *App {
	return newWithStores(config.Config{
		HTTPAddr:           ":0",
		SessionCookieName:  "cc_poker_session",
		SessionTTL:         time.Hour,
		CookieSecure:       false,
		AllowedOrigins:     map[string]struct{}{},
		DefaultTableID:     "table-1",
		SnapshotEnabled:    false,
		SnapshotTimeout:    time.Second,
		PostgresTimeout:    time.Second,
		GuestWalletInitial: 2000,
		AuthWalletInitial:  10000,
		SupabaseTimeout:    time.Second,
	}, log.New(io.Discard, "", 0), nil, nil, verifier)
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

func TestGuestSessionRejectsDuplicateNickname(t *testing.T) {
	app := newTestApp()
	server := httptest.NewServer(app.Handler())
	defer server.Close()

	firstBody := bytes.NewBufferString(`{"nickname":"UniqueNick01"}`)
	firstReq, err := http.NewRequest(http.MethodPost, server.URL+"/v1/session/guest", firstBody)
	if err != nil {
		t.Fatalf("failed to create first request: %v", err)
	}
	firstReq.Header.Set("Content-Type", "application/json")

	firstRes, err := http.DefaultClient.Do(firstReq)
	if err != nil {
		t.Fatalf("failed to send first request: %v", err)
	}
	defer firstRes.Body.Close()

	if firstRes.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected first status code: got=%d want=%d", firstRes.StatusCode, http.StatusCreated)
	}

	secondBody := bytes.NewBufferString(`{"nickname":"UniqueNick01"}`)
	secondReq, err := http.NewRequest(http.MethodPost, server.URL+"/v1/session/guest", secondBody)
	if err != nil {
		t.Fatalf("failed to create second request: %v", err)
	}
	secondReq.Header.Set("Content-Type", "application/json")

	secondRes, err := http.DefaultClient.Do(secondReq)
	if err != nil {
		t.Fatalf("failed to send second request: %v", err)
	}
	defer secondRes.Body.Close()

	if secondRes.StatusCode != http.StatusConflict {
		t.Fatalf("unexpected second status code: got=%d want=%d", secondRes.StatusCode, http.StatusConflict)
	}
}

func TestAuthExchangeRejectsUnverifiedEmail(t *testing.T) {
	app := newTestAppWithAuthVerifier(fakeAuthVerifier{
		user: auth.User{
			UserID:        "auth-user-1",
			Email:         "guest@example.com",
			EmailVerified: false,
		},
		err: auth.ErrEmailNotVerified,
	})

	reqBody := bytes.NewBufferString(`{"access_token":"dummy-token"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/exchange", reqBody)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	app.Handler().ServeHTTP(rec, req)
	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("unexpected status code: got=%d want=%d", res.StatusCode, http.StatusForbidden)
	}
}

func TestAuthExchangeSetsCookieAndUsesAuthWalletInitial(t *testing.T) {
	app := newTestAppWithAuthVerifier(fakeAuthVerifier{
		user: auth.User{
			UserID:        "4ec2f95f-7fd2-4f68-84f4-a8a173a0f3ce",
			Email:         "verified@example.com",
			EmailVerified: true,
			Nickname:      "VerifiedUser",
		},
	})
	server := httptest.NewServer(app.Handler())
	defer server.Close()

	reqBody := bytes.NewBufferString(`{"access_token":"dummy-token"}`)
	req, err := http.NewRequest(http.MethodPost, server.URL+"/v1/auth/exchange", reqBody)
	if err != nil {
		t.Fatalf("failed to create auth exchange request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to call auth exchange endpoint: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status code: got=%d want=%d", res.StatusCode, http.StatusCreated)
	}

	cookies := res.Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected session cookie in auth exchange response")
	}

	createBuyInExpectStatus(t, server.URL, cookies[0], "table-1", 2500, http.StatusCreated)
}

func TestAuthExchangeRejectsDuplicateNickname(t *testing.T) {
	app := newTestAppWithAuthVerifier(fakeAuthVerifier{
		user: auth.User{
			UserID:        "31cf4a14-87f3-4e8a-aee5-174339f3cbf3",
			Email:         "verified@example.com",
			EmailVerified: true,
			Nickname:      "DuplicatedNick",
		},
	})
	server := httptest.NewServer(app.Handler())
	defer server.Close()

	firstReqBody := bytes.NewBufferString(`{"nickname":"DuplicatedNick"}`)
	firstReq, err := http.NewRequest(http.MethodPost, server.URL+"/v1/session/guest", firstReqBody)
	if err != nil {
		t.Fatalf("failed to create guest request: %v", err)
	}
	firstReq.Header.Set("Content-Type", "application/json")

	firstRes, err := http.DefaultClient.Do(firstReq)
	if err != nil {
		t.Fatalf("failed to create guest session: %v", err)
	}
	defer firstRes.Body.Close()
	if firstRes.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected guest status code: got=%d want=%d", firstRes.StatusCode, http.StatusCreated)
	}

	secondReqBody := bytes.NewBufferString(`{"access_token":"dummy-token","nickname":"DuplicatedNick"}`)
	secondReq, err := http.NewRequest(http.MethodPost, server.URL+"/v1/auth/exchange", secondReqBody)
	if err != nil {
		t.Fatalf("failed to create auth exchange request: %v", err)
	}
	secondReq.Header.Set("Content-Type", "application/json")

	secondRes, err := http.DefaultClient.Do(secondReq)
	if err != nil {
		t.Fatalf("failed to call auth exchange endpoint: %v", err)
	}
	defer secondRes.Body.Close()

	if secondRes.StatusCode != http.StatusConflict {
		t.Fatalf("unexpected auth exchange status code: got=%d want=%d", secondRes.StatusCode, http.StatusConflict)
	}
}

func TestGuestWalletInitialIs2000(t *testing.T) {
	app := newTestApp()
	server := httptest.NewServer(app.Handler())
	defer server.Close()

	cookie := issueGuestCookie(t, server.URL)
	createBuyInExpectStatus(t, server.URL, cookie, "table-1", 2500, http.StatusConflict)
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

	createBuyIn(t, server.URL, cookie1, "table-1", 200)
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

	createBuyIn(t, server.URL, cookie2, "table-1", 200)
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

	createBuyIn(t, server.URL, cookie, "table-1", 200)
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

	next := atomic.AddUint64(&testGuestNicknameSeq, 1)
	nickname := "Kabigon-" + strconv.FormatUint(next, 10)
	reqBody := bytes.NewBufferString(`{"nickname":"` + nickname + `"}`)
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

var testGuestNicknameSeq uint64

func createBuyIn(t *testing.T, baseURL string, cookie *http.Cookie, tableID string, amount int64) {
	createBuyInExpectStatus(t, baseURL, cookie, tableID, amount, http.StatusCreated)
}

func createBuyInExpectStatus(t *testing.T, baseURL string, cookie *http.Cookie, tableID string, amount int64, expectedStatus int) {
	t.Helper()

	reqBody := bytes.NewBufferString(`{"table_id":"` + tableID + `","amount":` + strconv.FormatInt(amount, 10) + `}`)
	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/tables/buy-in", reqBody)
	if err != nil {
		t.Fatalf("failed to create buy-in request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to call buy-in endpoint: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != expectedStatus {
		t.Fatalf("unexpected buy-in status code: got=%d want=%d", res.StatusCode, expectedStatus)
	}
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

type fakeAuthVerifier struct {
	user auth.User
	err  error
}

func (f fakeAuthVerifier) VerifyAccessToken(context.Context, string) (auth.User, error) {
	if f.err != nil {
		return f.user, f.err
	}
	return f.user, nil
}
