package table

import "testing"

func TestCanStartWithMinimumPlayers(t *testing.T) {
	tbl := New("table-1")

	snapshot, _, err := tbl.Join("player-1", "Guest-1", nil)
	if err != nil {
		t.Fatalf("first join failed: %v", err)
	}

	if snapshot.CanStart {
		t.Fatalf("expected can_start=false with 1 player, got true")
	}

	snapshot, _, err = tbl.Join("player-2", "Guest-2", nil)
	if err != nil {
		t.Fatalf("second join failed: %v", err)
	}

	if !snapshot.CanStart {
		t.Fatalf("expected can_start=true with 2 players, got false")
	}
}

func TestRestoreSnapshot(t *testing.T) {
	tbl := New("table-1")
	input := Snapshot{
		TableID: "table-1",
		Players: []Player{
			{PlayerID: "ply-1", Nickname: "A", SeatIndex: 1},
			{PlayerID: "ply-2", Nickname: "B", SeatIndex: 5},
		},
	}

	if err := tbl.Restore(input, 11); err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	snapshot, seq := tbl.Snapshot()
	if seq != 11 {
		t.Fatalf("unexpected seq: got=%d want=11", seq)
	}
	if snapshot.ActivePlayers != 2 {
		t.Fatalf("unexpected active players: got=%d want=2", snapshot.ActivePlayers)
	}
	if !snapshot.CanStart {
		t.Fatalf("expected can_start=true with restored 2 players")
	}
}
