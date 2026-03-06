package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

type healthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Timestamp string `json:"timestamp"`
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)

	addr := getenv("CC_POKER_BACKEND_ADDR", ":8080")

	// 서버 시작 시 운영 로그에서 빠르게 식별할 수 있도록 기본 정보를 출력한다.
	log.Printf("cc-poker backend server listening on %s", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("failed to start http server: %v", err)
	}
}

// healthHandler는 서버 상태를 확인하기 위한 헬스체크 응답을 반환한다.
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	resp := healthResponse{
		Status:    "ok",
		Service:   "cc-poker-backend",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "failed to encode health response", http.StatusInternalServerError)
		return
	}
}

// getenv는 환경변수가 비어 있을 때 기본값을 반환한다.
func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
