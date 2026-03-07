package app

type guestSessionRequest struct {
	Nickname string `json:"nickname"`
}

type guestSessionResponse struct {
	PlayerID  string `json:"player_id"`
	Nickname  string `json:"nickname"`
	UserType  string `json:"user_type"`
	ExpiresAt string `json:"expires_at"`
}

type authExchangeRequest struct {
	AccessToken string `json:"access_token"`
	Nickname    string `json:"nickname"`
}

type authExchangeResponse struct {
	PlayerID      string `json:"player_id"`
	Nickname      string `json:"nickname"`
	UserType      string `json:"user_type"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	ExpiresAt     string `json:"expires_at"`
}

type currentSessionResponse struct {
	PlayerID      string `json:"player_id"`
	Nickname      string `json:"nickname"`
	UserType      string `json:"user_type"`
	Email         string `json:"email,omitempty"`
	EmailVerified bool   `json:"email_verified"`
	ExpiresAt     string `json:"expires_at"`
}

type nicknameCheckRequest struct {
	Nickname string `json:"nickname"`
}

type nicknameCheckResponse struct {
	Available bool `json:"available"`
}

type tableBuyInRequest struct {
	TableID string `json:"table_id"`
	Amount  int64  `json:"amount"`
}

type tableBuyInResponse struct {
	BuyInID      int64  `json:"buy_in_id"`
	PlayerID     string `json:"player_id"`
	TableID      string `json:"table_id"`
	RoomID       string `json:"room_id"`
	Amount       int64  `json:"amount"`
	BalanceAfter int64  `json:"balance_after"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
}

type walletResponse struct {
	PlayerID  string `json:"player_id"`
	UserType  string `json:"user_type"`
	Balance   int64  `json:"balance"`
	Timestamp string `json:"timestamp"`
}

type healthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Timestamp string `json:"timestamp"`
}
