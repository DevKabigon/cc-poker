package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	// ErrAuthDisabled는 인증 기능이 비활성화됐을 때 반환된다.
	ErrAuthDisabled = errors.New("auth disabled")
	// ErrInvalidAccessToken는 액세스 토큰이 유효하지 않을 때 반환된다.
	ErrInvalidAccessToken = errors.New("invalid access token")
	// ErrEmailNotVerified는 이메일 인증이 완료되지 않았을 때 반환된다.
	ErrEmailNotVerified = errors.New("email not verified")
)

// User는 외부 인증 공급자 검증 결과로 반환되는 최소 사용자 정보다.
type User struct {
	UserID        string
	Email         string
	EmailVerified bool
	Nickname      string
}

// Verifier는 외부 액세스 토큰을 검증해 사용자 정보를 반환한다.
type Verifier interface {
	VerifyAccessToken(ctx context.Context, accessToken string) (User, error)
}

// SupabaseConfig는 Supabase Auth 토큰 검증에 필요한 설정값이다.
type SupabaseConfig struct {
	Enabled bool
	URL     string
	AnonKey string
	Timeout time.Duration
}

// NewSupabaseVerifier는 Supabase 인증 검증기를 생성한다.
func NewSupabaseVerifier(cfg SupabaseConfig) Verifier {
	if !cfg.Enabled {
		return noopVerifier{}
	}

	baseURL := strings.TrimSpace(cfg.URL)
	anonKey := strings.TrimSpace(cfg.AnonKey)
	if baseURL == "" || anonKey == "" {
		return noopVerifier{}
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}

	return &supabaseVerifier{
		baseURL: strings.TrimRight(baseURL, "/"),
		anonKey: anonKey,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

type noopVerifier struct{}

func (noopVerifier) VerifyAccessToken(context.Context, string) (User, error) {
	return User{}, ErrAuthDisabled
}

type supabaseVerifier struct {
	baseURL string
	anonKey string
	client  *http.Client
}

type supabaseUserResponse struct {
	ID               string         `json:"id"`
	Email            string         `json:"email"`
	EmailConfirmedAt *string        `json:"email_confirmed_at"`
	ConfirmedAt      *string        `json:"confirmed_at"`
	UserMetadata     map[string]any `json:"user_metadata"`
}

func (v *supabaseVerifier) VerifyAccessToken(ctx context.Context, accessToken string) (User, error) {
	trimmedToken := strings.TrimSpace(accessToken)
	if trimmedToken == "" {
		return User{}, ErrInvalidAccessToken
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, v.baseURL+"/auth/v1/user", nil)
	if err != nil {
		return User{}, fmt.Errorf("failed to build supabase request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+trimmedToken)
	request.Header.Set("apikey", v.anonKey)
	request.Header.Set("Accept", "application/json")

	response, err := v.client.Do(request)
	if err != nil {
		return User{}, fmt.Errorf("failed to call supabase auth api: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return User{}, ErrInvalidAccessToken
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return User{}, fmt.Errorf("supabase auth api returned status=%d", response.StatusCode)
	}

	var payload supabaseUserResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return User{}, fmt.Errorf("failed to decode supabase user payload: %w", err)
	}
	if strings.TrimSpace(payload.ID) == "" {
		return User{}, ErrInvalidAccessToken
	}

	verified := false
	if payload.EmailConfirmedAt != nil && strings.TrimSpace(*payload.EmailConfirmedAt) != "" {
		verified = true
	}
	if !verified && payload.ConfirmedAt != nil && strings.TrimSpace(*payload.ConfirmedAt) != "" {
		verified = true
	}

	user := User{
		UserID:        payload.ID,
		Email:         strings.TrimSpace(payload.Email),
		EmailVerified: verified,
		Nickname:      extractNickname(payload.UserMetadata),
	}

	if !user.EmailVerified {
		return user, ErrEmailNotVerified
	}
	return user, nil
}

func extractNickname(metadata map[string]any) string {
	if len(metadata) == 0 {
		return ""
	}

	for _, key := range []string{"nickname", "name", "display_name"} {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		text, ok := value.(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(text)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
