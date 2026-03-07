package app

func (a *App) registerRoutes() {
	a.mux.HandleFunc("/health", a.handleHealth)
	a.mux.HandleFunc("/v1/session/guest", a.handleGuestSession)
	a.mux.HandleFunc("/v1/session/current", a.handleCurrentSession)
	a.mux.HandleFunc("/v1/auth/nickname/check", a.handleNicknameCheck)
	a.mux.HandleFunc("/v1/auth/exchange", a.handleAuthExchange)
	a.mux.HandleFunc("/v1/auth/logout", a.handleAuthLogout)
	a.mux.HandleFunc("/v1/wallet", a.handleWallet)
	a.mux.HandleFunc("/v1/tables/buy-in", a.handleTableBuyIn)
	a.mux.HandleFunc("/ws", a.handleWebSocket)
}
