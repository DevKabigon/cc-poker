package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	// ErrNicknameTaken은 이미 다른 플레이어가 사용하는 닉네임일 때 반환된다.
	ErrNicknameTaken = errors.New("nickname is already taken")
)

// PlayerSession은 플레이어의 인증 세션 정보를 나타낸다.
type PlayerSession struct {
	SessionID string
	PlayerID  string
	Nickname  string
	ExpiresAt time.Time
}

// Store는 메모리 기반 세션 저장소다.
type Store struct {
	mu              sync.RWMutex
	sessions        map[string]PlayerSession
	nicknameOwners  map[string]string
	playerNicknames map[string]string
	now             func() time.Time
}

// NewStore는 세션 저장소를 생성한다.
func NewStore(now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}

	return &Store{
		sessions:        make(map[string]PlayerSession),
		nicknameOwners:  make(map[string]string),
		playerNicknames: make(map[string]string),
		now:             now,
	}
}

// CreateGuest는 게스트 플레이어 세션을 생성하고 저장한다.
func (s *Store) CreateGuest(nickname string, ttl time.Duration) (PlayerSession, error) {
	playerID, err := newID("ply")
	if err != nil {
		return PlayerSession{}, fmt.Errorf("failed to generate player id: %w", err)
	}

	return s.Create(playerID, nickname, ttl)
}

// Create는 지정된 플레이어 ID로 세션을 생성하고 저장한다.
func (s *Store) Create(playerID, nickname string, ttl time.Duration) (PlayerSession, error) {
	sessionID, err := newID("ses")
	if err != nil {
		return PlayerSession{}, fmt.Errorf("failed to generate session id: %w", err)
	}

	session := PlayerSession{
		SessionID: sessionID,
		PlayerID:  playerID,
		Nickname:  normalizeNickname(nickname, playerID),
		ExpiresAt: s.now().Add(ttl),
	}

	s.mu.Lock()
	canonicalNickname := canonicalizeNickname(session.Nickname)
	if ownerPlayerID, exists := s.nicknameOwners[canonicalNickname]; exists && ownerPlayerID != playerID {
		s.mu.Unlock()
		return PlayerSession{}, ErrNicknameTaken
	}

	// 동일 플레이어의 닉네임 변경이 발생하면 이전 인덱스를 정리한다.
	if previousCanonical, exists := s.playerNicknames[playerID]; exists && previousCanonical != canonicalNickname {
		delete(s.nicknameOwners, previousCanonical)
	}

	s.nicknameOwners[canonicalNickname] = playerID
	s.playerNicknames[playerID] = canonicalNickname
	s.sessions[sessionID] = session
	s.mu.Unlock()

	return session, nil
}

// Delete는 세션 ID를 기준으로 메모리 세션을 제거한다.
func (s *Store) Delete(sessionID string) {
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.mu.Unlock()
}

// FindValid는 세션 ID로 유효한 세션을 조회한다.
func (s *Store) FindValid(sessionID string) (PlayerSession, bool) {
	s.mu.RLock()
	session, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if !ok {
		return PlayerSession{}, false
	}

	if !session.ExpiresAt.After(s.now()) {
		s.mu.Lock()
		delete(s.sessions, sessionID)
		s.mu.Unlock()
		return PlayerSession{}, false
	}

	return session, true
}

func newID(prefix string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	return prefix + "_" + hex.EncodeToString(buf), nil
}

func normalizeNickname(input, playerID string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed != "" {
		runes := []rune(trimmed)
		if len(runes) > 20 {
			return string(runes[:20])
		}
		return trimmed
	}

	suffix := playerID
	if len(suffix) > 6 {
		suffix = suffix[len(suffix)-6:]
	}
	return "Guest-" + suffix
}

func canonicalizeNickname(input string) string {
	return strings.ToLower(strings.TrimSpace(input))
}
