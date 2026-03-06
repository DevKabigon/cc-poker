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
