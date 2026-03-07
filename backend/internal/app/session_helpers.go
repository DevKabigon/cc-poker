package app

import (
	"net/http"
	"strings"
	"time"

	"github.com/DevKabigon/cc-poker/backend/internal/session"
)

func (a *App) authFromCookie(r *http.Request) (session.PlayerSession, bool) {
	cookie, err := r.Cookie(a.cfg.SessionCookieName)
	if err != nil || cookie.Value == "" {
		return session.PlayerSession{}, false
	}

	return a.sessions.FindValid(cookie.Value)
}

// revokeSessionFromCookie는 같은 브라우저에서 새 세션 발급 전에 기존 세션/WS를 정리한다.
func (a *App) revokeSessionFromCookie(r *http.Request) {
	cookie, err := r.Cookie(a.cfg.SessionCookieName)
	if err != nil {
		return
	}

	sessionID := strings.TrimSpace(cookie.Value)
	if sessionID == "" {
		return
	}

	playerSession, exists := a.sessions.FindValid(sessionID)
	a.sessions.Delete(sessionID)
	if !exists {
		return
	}

	// 동일 플레이어의 기존 WS 연결은 새 세션 발급 시 즉시 종료한다.
	for _, client := range a.hub.clientsForPlayer(playerSession.PlayerID) {
		a.closeClient(client)
	}
}

func (a *App) setSessionCookie(w http.ResponseWriter, playerSession session.PlayerSession) {
	ttl := int(time.Until(playerSession.ExpiresAt).Seconds())
	if ttl < 0 {
		ttl = 0
	}

	http.SetCookie(w, &http.Cookie{
		Name:     a.cfg.SessionCookieName,
		Value:    playerSession.SessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   ttl,
		Expires:  playerSession.ExpiresAt,
	})
}

func (a *App) checkOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}

	if len(a.cfg.AllowedOrigins) == 0 {
		return true
	}

	_, ok := a.cfg.AllowedOrigins[origin]
	return ok
}

func (a *App) resolveTableID(requestTableID string) string {
	trimmed := strings.TrimSpace(requestTableID)
	if trimmed == "" {
		return a.cfg.DefaultTableID
	}
	return trimmed
}
