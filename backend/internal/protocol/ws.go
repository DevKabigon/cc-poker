package protocol

import "encoding/json"

// ClientEnvelope은 클라이언트가 WS로 보내는 이벤트 기본 형식이다.
type ClientEnvelope struct {
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
}

// JoinTablePayload는 테이블 입장 요청 본문이다.
type JoinTablePayload struct {
	TableID   string `json:"table_id"`
	SeatIndex *int   `json:"seat_index,omitempty"`
}

// LeaveTablePayload는 테이블 퇴장 요청 본문이다.
type LeaveTablePayload struct {
	TableID string `json:"table_id"`
}

// ServerEnvelope는 서버가 WS로 보내는 이벤트 기본 형식이다.
type ServerEnvelope struct {
	EventVersion int    `json:"event_version"`
	EventType    string `json:"event_type"`
	TableID      string `json:"table_id,omitempty"`
	Seq          uint64 `json:"seq,omitempty"`
	SentAt       string `json:"sent_at"`
	Payload      any    `json:"payload"`
}

// ServerEnvelopeRaw는 테스트/검증용으로 payload를 RawMessage로 받는 형식이다.
type ServerEnvelopeRaw struct {
	EventVersion int             `json:"event_version"`
	EventType    string          `json:"event_type"`
	TableID      string          `json:"table_id,omitempty"`
	Seq          uint64          `json:"seq,omitempty"`
	SentAt       string          `json:"sent_at"`
	Payload      json.RawMessage `json:"payload"`
}

// SnapshotPlayer는 테이블 스냅샷의 플레이어 항목이다.
type SnapshotPlayer struct {
	PlayerID  string `json:"player_id"`
	Nickname  string `json:"nickname"`
	SeatIndex int    `json:"seat_index"`
}

// TableSnapshotPayload는 테이블 상태 전달 본문이다.
type TableSnapshotPayload struct {
	TableState        string           `json:"table_state"`
	MaxPlayers        int              `json:"max_players"`
	MinPlayersToStart int              `json:"min_players_to_start"`
	ActivePlayers     int              `json:"active_players"`
	CanStart          bool             `json:"can_start"`
	Players           []SnapshotPlayer `json:"players"`
}

// ErrorNoticePayload는 요청 오류를 클라이언트에 전달할 때 사용한다.
type ErrorNoticePayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
