package app

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/DevKabigon/cc-poker/backend/internal/auth"
	"github.com/DevKabigon/cc-poker/backend/internal/session"
	"github.com/DevKabigon/cc-poker/backend/internal/store"
	"github.com/DevKabigon/cc-poker/backend/internal/table"
)

func tableErrorToNotice(err error) (string, string) {
	switch {
	case errors.Is(err, table.ErrSeatInvalid):
		return "INVALID_SEAT", "seat index is invalid"
	case errors.Is(err, table.ErrSeatTaken):
		return "SEAT_TAKEN", "seat is already taken"
	case errors.Is(err, table.ErrTableFull):
		return "TABLE_FULL", "table is full"
	case errors.Is(err, table.ErrPlayerNotFound):
		return "PLAYER_NOT_FOUND", "player is not in table"
	default:
		return "INTERNAL_ERROR", fmt.Sprintf("internal error: %v", err)
	}
}

func buyInErrorToNotice(err error) (string, string) {
	switch {
	case errors.Is(err, store.ErrPendingBuyInNotFound):
		return "BUY_IN_REQUIRED", "buy-in is required before joining table"
	case errors.Is(err, store.ErrInvalidBuyInAmount):
		return "INVALID_BUY_IN", "buy-in amount is invalid"
	case errors.Is(err, store.ErrInsufficientBalance):
		return "INSUFFICIENT_BALANCE", "wallet balance is insufficient"
	case errors.Is(err, store.ErrTableNotFound):
		return "TABLE_NOT_FOUND", "table not found"
	default:
		return "INTERNAL_ERROR", fmt.Sprintf("internal error: %v", err)
	}
}

func buyInErrorToHTTP(err error) (int, string) {
	switch {
	case errors.Is(err, store.ErrTableNotFound):
		return http.StatusNotFound, "table not found"
	case errors.Is(err, store.ErrInvalidBuyInAmount):
		return http.StatusBadRequest, "buy-in amount is out of allowed range"
	case errors.Is(err, store.ErrInsufficientBalance):
		return http.StatusConflict, "insufficient wallet balance"
	default:
		return http.StatusInternalServerError, "failed to create buy-in"
	}
}

func authErrorToHTTP(err error) (int, string) {
	switch {
	case errors.Is(err, auth.ErrAuthDisabled):
		return http.StatusServiceUnavailable, "auth exchange is disabled"
	case errors.Is(err, auth.ErrInvalidAccessToken):
		return http.StatusUnauthorized, "invalid access token"
	case errors.Is(err, auth.ErrEmailNotVerified):
		return http.StatusForbidden, "email verification is required"
	default:
		return http.StatusBadGateway, "failed to verify external auth token"
	}
}

func nicknameErrorToHTTP(err error) (int, string) {
	switch {
	case errors.Is(err, session.ErrNicknameTaken), errors.Is(err, store.ErrNicknameTaken):
		return http.StatusConflict, "nickname is already taken"
	default:
		return http.StatusInternalServerError, "failed to create session"
	}
}
