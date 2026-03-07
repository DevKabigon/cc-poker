package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/DevKabigon/cc-poker/backend/internal/session"
)

// handleHealth는 서버 생존 상태를 확인하기 위한 엔드포인트다.
func (a *App) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:    "ok",
		Service:   "cc-poker-backend",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// handleNicknameCheck은 닉네임 사용 가능 여부를 검사한다.
func (a *App) handleNicknameCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req nicknameCheckRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}
	}

	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" {
		http.Error(w, "nickname is required", http.StatusBadRequest)
		return
	}

	if a.sessions.IsNicknameTaken(nickname) {
		writeJSON(w, http.StatusOK, nicknameCheckResponse{Available: false})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.PostgresTimeout)
	defer cancel()

	isTaken, err := a.eventStore.IsNicknameTaken(ctx, nickname)
	if err != nil {
		a.logger.Printf("failed to check nickname availability: nickname=%s err=%v", nickname, err)
		http.Error(w, "failed to check nickname", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, nicknameCheckResponse{Available: !isTaken})
}

// handleGuestSession은 게스트 세션을 발급하고 쿠키를 설정한다.
func (a *App) handleGuestSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req guestSessionRequest
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}
	}
	// 게스트 입장은 닉네임을 반드시 입력하도록 강제한다.
	if strings.TrimSpace(req.Nickname) == "" {
		http.Error(w, "nickname is required", http.StatusBadRequest)
		return
	}
	a.revokeSessionFromCookie(r)

	created, err := a.sessions.CreateGuest(req.Nickname, a.cfg.SessionTTL)
	if err != nil {
		if errors.Is(err, session.ErrNicknameTaken) {
			http.Error(w, "nickname is already taken", http.StatusConflict)
			return
		}
		a.logger.Printf("failed to create guest session: %v", err)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	if err := a.persistSession(created); err != nil {
		a.sessions.Delete(created.SessionID)
		statusCode, message := nicknameErrorToHTTP(err)
		http.Error(w, message, statusCode)
		return
	}
	a.ensureWallet(created.PlayerID, a.cfg.GuestWalletInitial)
	a.setSessionCookie(w, created)
	writeJSON(w, http.StatusCreated, guestSessionResponse{
		PlayerID:  created.PlayerID,
		Nickname:  created.Nickname,
		UserType:  created.UserType,
		ExpiresAt: created.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

// handleCurrentSession은 세션 쿠키 기준으로 현재 로그인 정보를 반환한다.
func (a *App) handleCurrentSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	playerSession, ok := a.authFromCookie(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	writeJSON(w, http.StatusOK, currentSessionResponse{
		PlayerID:      playerSession.PlayerID,
		Nickname:      playerSession.Nickname,
		UserType:      playerSession.UserType,
		Email:         playerSession.Email,
		EmailVerified: playerSession.EmailVerified,
		ExpiresAt:     playerSession.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

// handleAuthExchange는 Supabase access token을 서버 세션 쿠키로 교환한다.
func (a *App) handleAuthExchange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req authExchangeRequest
	if r.Body == nil {
		http.Error(w, "request body is required", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.SupabaseTimeout)
	defer cancel()

	authUser, err := a.authVerifier.VerifyAccessToken(ctx, req.AccessToken)
	if err != nil {
		statusCode, message := authErrorToHTTP(err)
		http.Error(w, message, statusCode)
		return
	}
	a.revokeSessionFromCookie(r)

	playerID := authUserToPlayerID(authUser.UserID)
	nickname := resolveAuthNickname(req.Nickname, authUser)
	created, err := a.sessions.Create(playerID, nickname, a.cfg.SessionTTL)
	if err != nil {
		if errors.Is(err, session.ErrNicknameTaken) {
			http.Error(w, "nickname is already taken", http.StatusConflict)
			return
		}
		a.logger.Printf("failed to create auth session: %v", err)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}
	created.UserType = "auth"
	created.Email = strings.TrimSpace(authUser.Email)
	created.EmailVerified = authUser.EmailVerified

	if err := a.persistSession(created); err != nil {
		a.sessions.Delete(created.SessionID)
		statusCode, message := nicknameErrorToHTTP(err)
		http.Error(w, message, statusCode)
		return
	}
	a.ensureWallet(created.PlayerID, a.cfg.AuthWalletInitial)
	a.setSessionCookie(w, created)

	writeJSON(w, http.StatusCreated, authExchangeResponse{
		PlayerID:      created.PlayerID,
		Nickname:      created.Nickname,
		UserType:      created.UserType,
		Email:         authUser.Email,
		EmailVerified: authUser.EmailVerified,
		ExpiresAt:     created.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

// handleAuthLogout은 세션 쿠키를 만료시키고 메모리 세션을 제거한다.
func (a *App) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie(a.cfg.SessionCookieName)
	if err == nil && strings.TrimSpace(cookie.Value) != "" {
		a.sessions.Delete(cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     a.cfg.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
	w.WriteHeader(http.StatusNoContent)
}

// handleWallet은 인증된 플레이어의 현재 지갑 잔액을 반환한다.
func (a *App) handleWallet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	playerSession, ok := a.authFromCookie(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	initialBalance := a.cfg.GuestWalletInitial
	if strings.EqualFold(playerSession.UserType, "auth") {
		initialBalance = a.cfg.AuthWalletInitial
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.PostgresTimeout)
	defer cancel()

	if err := a.eventStore.EnsureWallet(ctx, playerSession.PlayerID, initialBalance); err != nil {
		a.logger.Printf("failed to ensure wallet before query: player=%s err=%v", playerSession.PlayerID, err)
		http.Error(w, "failed to ensure wallet", http.StatusInternalServerError)
		return
	}

	balance, err := a.eventStore.GetWalletBalance(ctx, playerSession.PlayerID)
	if err != nil {
		a.logger.Printf("failed to query wallet balance: player=%s err=%v", playerSession.PlayerID, err)
		http.Error(w, "failed to query wallet", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, walletResponse{
		PlayerID:  playerSession.PlayerID,
		UserType:  playerSession.UserType,
		Balance:   balance,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// handleTableBuyIn은 쿠키 인증된 플레이어의 테이블 바이인을 생성한다.
func (a *App) handleTableBuyIn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	playerSession, ok := a.authFromCookie(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req tableBuyInRequest
	if r.Body == nil {
		http.Error(w, "request body is required", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	tableID := a.resolveTableID(req.TableID)
	if req.Amount <= 0 {
		http.Error(w, "buy-in amount must be positive", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.cfg.PostgresTimeout)
	defer cancel()

	receipt, err := a.eventStore.CreateBuyIn(ctx, playerSession.PlayerID, tableID, req.Amount)
	if err != nil {
		statusCode, message := buyInErrorToHTTP(err)
		http.Error(w, message, statusCode)
		return
	}

	writeJSON(w, http.StatusCreated, tableBuyInResponse{
		BuyInID:      receipt.BuyInID,
		PlayerID:     receipt.PlayerID,
		TableID:      receipt.TableID,
		RoomID:       receipt.RoomID,
		Amount:       receipt.Amount,
		BalanceAfter: receipt.BalanceAfter,
		Status:       receipt.Status,
		CreatedAt:    receipt.CreatedAt.UTC().Format(time.RFC3339),
	})
}
