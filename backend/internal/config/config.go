package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPAddr           = ":8080"
	defaultSessionCookieName  = "cc_poker_session"
	defaultSessionTTL         = 24 * time.Hour
	defaultCookieSecure       = false
	defaultTableID            = "room_1_2_table_1"
	defaultRedisAddr          = "127.0.0.1:6379"
	defaultRedisPassword      = ""
	defaultRedisDB            = 0
	defaultRedisKeyPrefix     = "cc_poker"
	defaultSnapshotEnabled    = true
	defaultSnapshotTimeout    = 500 * time.Millisecond
	defaultPostgresEnabled    = true
	defaultPostgresDSN        = "postgres://ccpoker:ccpoker@127.0.0.1:5432/ccpoker?sslmode=disable"
	defaultPostgresMaxConns   = 10
	defaultPostgresTimeout    = 800 * time.Millisecond
	defaultGuestWalletInitial = 2000
	defaultAuthWalletInitial  = 10000
	defaultSupabaseEnabled    = false
	defaultSupabaseURL        = ""
	defaultSupabaseAnonKey    = ""
	defaultSupabaseTimeout    = 1500 * time.Millisecond
)

// Config는 백엔드 서버 실행에 필요한 설정값 묶음이다.
type Config struct {
	HTTPAddr           string
	SessionCookieName  string
	SessionTTL         time.Duration
	CookieSecure       bool
	AllowedOrigins     map[string]struct{}
	DefaultTableID     string
	RedisAddr          string
	RedisPassword      string
	RedisDB            int
	RedisKeyPrefix     string
	SnapshotEnabled    bool
	SnapshotTimeout    time.Duration
	PostgresEnabled    bool
	PostgresDSN        string
	PostgresMaxConns   int
	PostgresTimeout    time.Duration
	GuestWalletInitial int64
	AuthWalletInitial  int64
	SupabaseEnabled    bool
	SupabaseURL        string
	SupabaseAnonKey    string
	SupabaseTimeout    time.Duration
}

// Load는 환경변수 기반 설정을 읽고 기본값을 채워 반환한다.
func Load() Config {
	loadDotEnvIfPresent()

	return Config{
		HTTPAddr:           getenv("CC_POKER_BACKEND_ADDR", defaultHTTPAddr),
		SessionCookieName:  getenv("CC_POKER_SESSION_COOKIE_NAME", defaultSessionCookieName),
		SessionTTL:         time.Duration(getenvInt("CC_POKER_SESSION_TTL_SECONDS", int(defaultSessionTTL.Seconds()))) * time.Second,
		CookieSecure:       getenvBool("CC_POKER_COOKIE_SECURE", defaultCookieSecure),
		AllowedOrigins:     parseAllowedOrigins(getenv("CC_POKER_ALLOWED_ORIGINS", "")),
		DefaultTableID:     getenv("CC_POKER_DEFAULT_TABLE_ID", defaultTableID),
		RedisAddr:          getenv("CC_POKER_REDIS_ADDR", defaultRedisAddr),
		RedisPassword:      getenv("CC_POKER_REDIS_PASSWORD", defaultRedisPassword),
		RedisDB:            getenvIntAllowZero("CC_POKER_REDIS_DB", defaultRedisDB),
		RedisKeyPrefix:     getenv("CC_POKER_REDIS_KEY_PREFIX", defaultRedisKeyPrefix),
		SnapshotEnabled:    getenvBool("CC_POKER_SNAPSHOT_ENABLED", defaultSnapshotEnabled),
		SnapshotTimeout:    time.Duration(getenvInt("CC_POKER_SNAPSHOT_TIMEOUT_MS", int(defaultSnapshotTimeout.Milliseconds()))) * time.Millisecond,
		PostgresEnabled:    getenvBool("CC_POKER_POSTGRES_ENABLED", defaultPostgresEnabled),
		PostgresDSN:        getenv("CC_POKER_POSTGRES_DSN", defaultPostgresDSN),
		PostgresMaxConns:   getenvInt("CC_POKER_POSTGRES_MAX_CONNS", defaultPostgresMaxConns),
		PostgresTimeout:    time.Duration(getenvInt("CC_POKER_POSTGRES_TIMEOUT_MS", int(defaultPostgresTimeout.Milliseconds()))) * time.Millisecond,
		GuestWalletInitial: int64(getenvInt("CC_POKER_GUEST_WALLET_INITIAL", defaultGuestWalletInitial)),
		AuthWalletInitial:  int64(getenvInt("CC_POKER_AUTH_WALLET_INITIAL", defaultAuthWalletInitial)),
		SupabaseEnabled:    getenvBool("CC_POKER_SUPABASE_ENABLED", defaultSupabaseEnabled),
		SupabaseURL:        getenv("CC_POKER_SUPABASE_URL", defaultSupabaseURL),
		SupabaseAnonKey:    getenv("CC_POKER_SUPABASE_ANON_KEY", defaultSupabaseAnonKey),
		SupabaseTimeout:    time.Duration(getenvInt("CC_POKER_SUPABASE_TIMEOUT_MS", int(defaultSupabaseTimeout.Milliseconds()))) * time.Millisecond,
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

func getenvIntAllowZero(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
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
