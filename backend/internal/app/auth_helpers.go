package app

import (
	"strings"

	"github.com/DevKabigon/cc-poker/backend/internal/auth"
)

func authUserToPlayerID(authUserID string) string {
	trimmed := strings.TrimSpace(authUserID)
	normalized := strings.ReplaceAll(trimmed, "-", "")
	if normalized == "" {
		return "usr_unknown"
	}
	return "usr_" + normalized
}

func resolveAuthNickname(requestNickname string, authUser auth.User) string {
	candidates := []string{
		requestNickname,
		authUser.Nickname,
		emailLocalPart(authUser.Email),
		fallbackAuthNickname(authUser.UserID),
	}
	for _, candidate := range candidates {
		trimmed := strings.TrimSpace(candidate)
		if trimmed == "" {
			continue
		}

		runes := []rune(trimmed)
		if len(runes) > 20 {
			return string(runes[:20])
		}
		return trimmed
	}
	return "User-000001"
}

func emailLocalPart(email string) string {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return ""
	}

	atIndex := strings.Index(trimmed, "@")
	if atIndex <= 0 {
		return ""
	}
	return trimmed[:atIndex]
}

func fallbackAuthNickname(userID string) string {
	trimmed := strings.TrimSpace(userID)
	trimmed = strings.ReplaceAll(trimmed, "-", "")
	if len(trimmed) >= 6 {
		return "User-" + trimmed[:6]
	}
	if trimmed != "" {
		return "User-" + trimmed
	}
	return ""
}
