package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DevKabigon/cc-poker/backend/internal/app"
	"github.com/DevKabigon/cc-poker/backend/internal/config"
)

func main() {
	cfg := config.Load()
	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC)
	logger.Printf(
		"config loaded: supabase_enabled=%t supabase_url_set=%t supabase_key_set=%t",
		cfg.SupabaseEnabled,
		cfg.SupabaseURL != "",
		cfg.SupabaseAnonKey != "",
	)
	application := app.New(cfg, logger)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           application.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errorCh := make(chan error, 1)
	go func() {
		logger.Printf("cc-poker backend server listening on %s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errorCh <- err
		}
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case received := <-signalCh:
		logger.Printf("shutdown signal received: %s", received.String())
	case err := <-errorCh:
		logger.Fatalf("server error: %v", err)
	}

	// 종료 시 연결을 정리할 시간을 주어 요청 중단을 최소화한다.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Printf("graceful shutdown failed: %v", err)
		if closeErr := server.Close(); closeErr != nil {
			logger.Printf("forced close failed: %v", closeErr)
		}
	}
}
