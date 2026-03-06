package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/DevKabigon/cc-poker/backend/internal/session"
)

// TableEvent는 테이블 상태 변경 이벤트의 DB 저장 모델이다.
type TableEvent struct {
	TableID   string
	Seq       uint64
	EventType string
	PlayerID  string
	Payload   json.RawMessage
	CreatedAt time.Time
}

// EventStore는 세션/테이블 이벤트의 영속화를 담당한다.
type EventStore interface {
	SaveSession(ctx context.Context, playerSession session.PlayerSession) error
	SaveTableEvent(ctx context.Context, event TableEvent) error
}

// NewNoopEventStore는 저장 동작을 수행하지 않는 기본 구현체를 반환한다.
func NewNoopEventStore() EventStore {
	return noopEventStore{}
}

type noopEventStore struct{}

func (noopEventStore) SaveSession(context.Context, session.PlayerSession) error {
	return nil
}

func (noopEventStore) SaveTableEvent(context.Context, TableEvent) error {
	return nil
}
