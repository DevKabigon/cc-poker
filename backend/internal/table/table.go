package table

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

const (
	// MinPlayersToStart는 핸드를 시작하기 위한 최소 인원이다.
	MinPlayersToStart = 2
	// MaxPlayers는 테이블 최대 인원이다.
	MaxPlayers = 9
)

var (
	// ErrTableFull은 테이블이 가득 찼을 때 반환된다.
	ErrTableFull = errors.New("table is full")
	// ErrSeatInvalid는 요청 좌석 인덱스가 범위를 벗어났을 때 반환된다.
	ErrSeatInvalid = errors.New("seat index is invalid")
	// ErrSeatTaken은 요청 좌석이 이미 점유된 경우 반환된다.
	ErrSeatTaken = errors.New("seat is already taken")
	// ErrPlayerNotFound는 플레이어가 테이블에 없을 때 반환된다.
	ErrPlayerNotFound = errors.New("player is not seated")
)

// Player는 테이블 좌석에 앉은 플레이어 정보를 나타낸다.
type Player struct {
	PlayerID  string `json:"player_id"`
	Nickname  string `json:"nickname"`
	SeatIndex int    `json:"seat_index"`
}

// Snapshot은 테이블 현재 상태를 직렬화 가능한 형태로 표현한다.
type Snapshot struct {
	TableID           string   `json:"table_id"`
	MaxPlayers        int      `json:"max_players"`
	MinPlayersToStart int      `json:"min_players_to_start"`
	ActivePlayers     int      `json:"active_players"`
	CanStart          bool     `json:"can_start"`
	Players           []Player `json:"players"`
}

// Table은 단일 테이블 상태를 관리한다.
type Table struct {
	id         string
	mu         sync.Mutex
	seats      [MaxPlayers]*Player
	playerSeat map[string]int
	seq        uint64
}

// New는 새 테이블 인스턴스를 생성한다.
func New(tableID string) *Table {
	return &Table{
		id:         tableID,
		playerSeat: make(map[string]int),
	}
}

// Join은 플레이어를 좌석에 배치하고 최신 스냅샷을 반환한다.
func (t *Table) Join(playerID, nickname string, preferredSeat *int) (Snapshot, uint64, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.playerSeat[playerID]; ok {
		snapshot := t.snapshotLocked()
		return snapshot, t.seq, nil
	}

	seatIndex, err := t.allocateSeatLocked(preferredSeat)
	if err != nil {
		return Snapshot{}, t.seq, err
	}

	player := &Player{
		PlayerID:  playerID,
		Nickname:  nickname,
		SeatIndex: seatIndex,
	}
	t.seats[seatIndex] = player
	t.playerSeat[playerID] = seatIndex
	t.seq++

	return t.snapshotLocked(), t.seq, nil
}

// Leave는 플레이어를 테이블에서 제거하고 최신 스냅샷을 반환한다.
func (t *Table) Leave(playerID string) (Snapshot, uint64, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	seatIndex, ok := t.playerSeat[playerID]
	if !ok {
		return Snapshot{}, t.seq, ErrPlayerNotFound
	}

	t.seats[seatIndex] = nil
	delete(t.playerSeat, playerID)
	t.seq++

	return t.snapshotLocked(), t.seq, nil
}

// Snapshot은 현재 테이블 상태를 반환한다.
func (t *Table) Snapshot() (Snapshot, uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.snapshotLocked(), t.seq
}

func (t *Table) allocateSeatLocked(preferredSeat *int) (int, error) {
	if preferredSeat != nil {
		if *preferredSeat < 0 || *preferredSeat >= MaxPlayers {
			return 0, ErrSeatInvalid
		}
		if t.seats[*preferredSeat] != nil {
			return 0, ErrSeatTaken
		}
		return *preferredSeat, nil
	}

	for idx := 0; idx < MaxPlayers; idx++ {
		if t.seats[idx] == nil {
			return idx, nil
		}
	}

	return 0, ErrTableFull
}

func (t *Table) snapshotLocked() Snapshot {
	players := make([]Player, 0, MaxPlayers)
	for _, seat := range t.seats {
		if seat == nil {
			continue
		}
		players = append(players, *seat)
	}

	sort.Slice(players, func(i, j int) bool {
		return players[i].SeatIndex < players[j].SeatIndex
	})

	activePlayers := len(players)
	return Snapshot{
		TableID:           t.id,
		MaxPlayers:        MaxPlayers,
		MinPlayersToStart: MinPlayersToStart,
		ActivePlayers:     activePlayers,
		CanStart:          activePlayers >= MinPlayersToStart,
		Players:           players,
	}
}

func (t *Table) String() string {
	snapshot, _ := t.Snapshot()
	return fmt.Sprintf("table=%s active=%d can_start=%v", snapshot.TableID, snapshot.ActivePlayers, snapshot.CanStart)
}
