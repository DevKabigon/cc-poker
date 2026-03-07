package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
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

// BuyInReceipt는 바이인 생성/소비 결과를 표현한다.
type BuyInReceipt struct {
	BuyInID      int64
	PlayerID     string
	TableID      string
	RoomID       string
	Amount       int64
	BalanceAfter int64
	Status       string
	CreatedAt    time.Time
}

var (
	// ErrNicknameTaken은 이미 사용 중인 닉네임으로 저장하려고 할 때 반환된다.
	ErrNicknameTaken = errors.New("nickname is already taken")
	// ErrTableNotFound는 테이블이 존재하지 않을 때 반환된다.
	ErrTableNotFound = errors.New("table not found")
	// ErrInvalidBuyInAmount는 바이인 금액이 룸 규칙을 벗어났을 때 반환된다.
	ErrInvalidBuyInAmount = errors.New("invalid buy-in amount")
	// ErrInsufficientBalance는 지갑 잔액이 부족할 때 반환된다.
	ErrInsufficientBalance = errors.New("insufficient balance")
	// ErrPendingBuyInNotFound는 소비 가능한 바이인이 없을 때 반환된다.
	ErrPendingBuyInNotFound = errors.New("pending buy-in not found")
)

// EventStore는 세션/테이블 이벤트의 영속화를 담당한다.
type EventStore interface {
	SeedRoomsAndTables(ctx context.Context) error
	EnsureWallet(ctx context.Context, playerID string, initialBalance int64) error
	CreateBuyIn(ctx context.Context, playerID, tableID string, amount int64) (BuyInReceipt, error)
	ConsumePendingBuyIn(ctx context.Context, playerID, tableID string) (BuyInReceipt, error)
	IsNicknameTaken(ctx context.Context, nickname string) (bool, error)
	SaveSession(ctx context.Context, playerSession session.PlayerSession) error
	SaveTableEvent(ctx context.Context, event TableEvent) error
}

// NewNoopEventStore는 저장 동작을 수행하지 않는 기본 구현체를 반환한다.
func NewNoopEventStore() EventStore {
	return &noopEventStore{
		pending:  make(map[string]BuyInReceipt),
		balances: make(map[string]int64),
	}
}

type noopEventStore struct {
	mu          sync.Mutex
	nextBuyInID int64
	pending     map[string]BuyInReceipt
	balances    map[string]int64
}

func (*noopEventStore) SeedRoomsAndTables(context.Context) error {
	return nil
}

func (n *noopEventStore) EnsureWallet(_ context.Context, playerID string, initialBalance int64) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if _, exists := n.balances[playerID]; !exists {
		n.balances[playerID] = initialBalance
	}
	return nil
}

func (n *noopEventStore) CreateBuyIn(_ context.Context, playerID, tableID string, amount int64) (BuyInReceipt, error) {
	tableID = strings.TrimSpace(tableID)
	if tableID == "" {
		return BuyInReceipt{}, ErrTableNotFound
	}
	if amount <= 0 {
		return BuyInReceipt{}, ErrInvalidBuyInAmount
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	key := buyInKey(playerID, tableID)
	if existing, ok := n.pending[key]; ok {
		existing.BalanceAfter = n.balances[playerID]
		return existing, nil
	}

	balance, ok := n.balances[playerID]
	if !ok {
		balance = 0
		n.balances[playerID] = balance
	}
	if balance < amount {
		return BuyInReceipt{}, ErrInsufficientBalance
	}

	n.nextBuyInID++
	updatedBalance := balance - amount
	n.balances[playerID] = updatedBalance

	receipt := BuyInReceipt{
		BuyInID:      n.nextBuyInID,
		PlayerID:     playerID,
		TableID:      tableID,
		RoomID:       "local_room",
		Amount:       amount,
		BalanceAfter: updatedBalance,
		Status:       "pending",
		CreatedAt:    time.Now().UTC(),
	}
	n.pending[key] = receipt
	return receipt, nil
}

func (n *noopEventStore) ConsumePendingBuyIn(_ context.Context, playerID, tableID string) (BuyInReceipt, error) {
	tableID = strings.TrimSpace(tableID)
	if tableID == "" {
		return BuyInReceipt{}, ErrPendingBuyInNotFound
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	key := buyInKey(playerID, tableID)
	receipt, ok := n.pending[key]
	if !ok {
		return BuyInReceipt{}, ErrPendingBuyInNotFound
	}

	delete(n.pending, key)
	receipt.Status = "consumed"
	receipt.BalanceAfter = n.balances[playerID]
	return receipt, nil
}

func (*noopEventStore) SaveSession(context.Context, session.PlayerSession) error {
	return nil
}

func (*noopEventStore) IsNicknameTaken(context.Context, string) (bool, error) {
	return false, nil
}

func (*noopEventStore) SaveTableEvent(context.Context, TableEvent) error {
	return nil
}

func buyInKey(playerID, tableID string) string {
	return playerID + "::" + tableID
}
