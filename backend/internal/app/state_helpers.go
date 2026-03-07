package app

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/DevKabigon/cc-poker/backend/internal/protocol"
	"github.com/DevKabigon/cc-poker/backend/internal/session"
	"github.com/DevKabigon/cc-poker/backend/internal/store"
	"github.com/DevKabigon/cc-poker/backend/internal/table"
)

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

func (a *App) currentPlayerStack(tableID, playerID string) (int64, bool) {
	snapshot, _ := a.tables.Snapshot(tableID)
	for _, seatedPlayer := range snapshot.Players {
		if seatedPlayer.PlayerID == playerID {
			return seatedPlayer.Stack, true
		}
	}
	return 0, false
}

// refundStackToWallet는 테이블 퇴장 시 남은 스택을 월렛으로 환급한다.
func (a *App) refundStackToWallet(playerID, tableID string, amount int64) {
	if amount <= 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.PostgresTimeout)
	defer cancel()

	updatedBalance, err := a.eventStore.CreditWallet(ctx, playerID, amount)
	if err != nil {
		a.logger.Printf("failed to refund stack to wallet: player=%s table=%s amount=%d err=%v", playerID, tableID, amount, err)
		return
	}

	a.logger.Printf("refunded stack to wallet: player=%s table=%s amount=%d balance_after=%d", playerID, tableID, amount, updatedBalance)
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
