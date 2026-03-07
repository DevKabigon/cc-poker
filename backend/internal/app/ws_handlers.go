package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/DevKabigon/cc-poker/backend/internal/protocol"
	"github.com/DevKabigon/cc-poker/backend/internal/table"
	"github.com/gorilla/websocket"
)

const wsEventVersion = 1

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
		stack, hasStack := a.currentPlayerStack(tableID, client.PlayerID())
		snapshot, seq, err := a.tables.Leave(tableID, client.PlayerID())
		if err != nil && !errors.Is(err, table.ErrPlayerNotFound) {
			a.logger.Printf("failed to leave table on disconnect: player=%s table=%s err=%v", client.PlayerID(), tableID, err)
		}

		if err == nil {
			if hasStack {
				a.refundStackToWallet(client.PlayerID(), tableID, stack)
			}
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
		previousStack, hadPreviousStack := a.currentPlayerStack(currentTableID, client.PlayerID())
		previousSnapshot, previousSeq, leaveErr := a.tables.Leave(currentTableID, client.PlayerID())
		if leaveErr != nil && !errors.Is(leaveErr, table.ErrPlayerNotFound) {
			a.refundStackToWallet(client.PlayerID(), tableID, stack)
			return nil, leaveErr
		}
		client.SetTableState(currentTableID, false)
		if leaveErr == nil {
			if hadPreviousStack {
				a.refundStackToWallet(client.PlayerID(), currentTableID, previousStack)
			}
			a.broadcastSnapshot(currentTableID, previousSnapshot, previousSeq)
			a.persistSnapshot(previousSnapshot, previousSeq)
			a.persistTableEvent("switch_leave", client.PlayerID(), previousSnapshot, previousSeq)
		}
	}

	snapshot, seq, err := a.tables.Join(tableID, client.PlayerID(), client.Nickname(), stack, joinReq.SeatIndex)
	if err != nil {
		a.refundStackToWallet(client.PlayerID(), tableID, stack)
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

	stack, hasStack := a.currentPlayerStack(tableID, client.PlayerID())
	snapshot, seq, err := a.tables.Leave(tableID, client.PlayerID())
	if err != nil {
		code, message := tableErrorToNotice(err)
		return a.sendErrorNotice(client, code, message)
	}

	if hasStack {
		a.refundStackToWallet(client.PlayerID(), tableID, stack)
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
