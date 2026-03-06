package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPAddr          = ":8080"
	defaultSessionCookieName = "cc_poker_session"
	defaultSessionTTL        = 24 * time.Hour
	defaultCookieSecure      = false
	defaultTableID           = "table-1"
)

// Config는 백엔드 서버 실행에 필요한 설정값 묶음이다.
type Config struct {
	HTTPAddr          string
	SessionCookieName string
	SessionTTL        time.Duration
	CookieSecure      bool
	AllowedOrigins    map[string]struct{}
	DefaultTableID    string
}

// Load는 환경변수 기반 설정을 읽고 기본값을 채워 반환한다.
func Load() Config {
	return Config{
		HTTPAddr:          getenv("CC_POKER_BACKEND_ADDR", defaultHTTPAddr),
		SessionCookieName: getenv("CC_POKER_SESSION_COOKIE_NAME", defaultSessionCookieName),
		SessionTTL:        time.Duration(getenvInt("CC_POKER_SESSION_TTL_SECONDS", int(defaultSessionTTL.Seconds()))) * time.Second,
		CookieSecure:      getenvBool("CC_POKER_COOKIE_SECURE", defaultCookieSecure),
		AllowedOrigins:    parseAllowedOrigins(getenv("CC_POKER_ALLOWED_ORIGINS", "")),
		DefaultTableID:    getenv("CC_POKER_DEFAULT_TABLE_ID", defaultTableID),
	}
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getenvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func getenvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseAllowedOrigins(raw string) map[string]struct{} {
	out := make(map[string]struct{})

	for _, item := range strings.Split(raw, ",") {
		origin := strings.TrimSpace(item)
		if origin == "" {
			continue
		}
		out[origin] = struct{}{}
	}

	return out
}
