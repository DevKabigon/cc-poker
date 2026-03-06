package table

import "sync"

// Manager는 테이블 인스턴스를 관리한다.
type Manager struct {
	mu     sync.Mutex
	tables map[string]*Table
}

// NewManager는 기본 테이블을 포함한 매니저를 생성한다.
func NewManager(defaultTableID string) *Manager {
	m := &Manager{
		tables: make(map[string]*Table),
	}
	m.tables[defaultTableID] = New(defaultTableID)
	return m
}

// Join은 지정된 테이블에 플레이어를 입장시킨다.
func (m *Manager) Join(tableID, playerID, nickname string, preferredSeat *int) (Snapshot, uint64, error) {
	table := m.getOrCreateTable(tableID)
	return table.Join(playerID, nickname, preferredSeat)
}

// Leave는 지정된 테이블에서 플레이어를 퇴장시킨다.
func (m *Manager) Leave(tableID, playerID string) (Snapshot, uint64, error) {
	table := m.getOrCreateTable(tableID)
	return table.Leave(playerID)
}

// Snapshot은 지정된 테이블의 현재 상태를 조회한다.
func (m *Manager) Snapshot(tableID string) (Snapshot, uint64) {
	table := m.getOrCreateTable(tableID)
	return table.Snapshot()
}

func (m *Manager) getOrCreateTable(tableID string) *Table {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.tables[tableID]
	if ok {
		return existing
	}

	created := New(tableID)
	m.tables[tableID] = created
	return created
}
