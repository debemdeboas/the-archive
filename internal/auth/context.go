package auth

import (
	"context"

	"github.com/debemdeboas/the-archive/internal/model"
)

// ContextKey is a type for context keys to avoid collisions
type ContextKey string

// ContextKeyUserId is the key for user ID in request context
const ContextKeyUserId ContextKey = "userID"

// ContextWithUserId returns a new context with the user ID set
func ContextWithUserId(ctx context.Context, userID model.UserId) context.Context {
	return context.WithValue(ctx, ContextKeyUserId, userID)
}

// UserIdFromContext extracts the user ID from context
func UserIdFromContext(ctx context.Context) (model.UserId, bool) {
	userID, ok := ctx.Value(ContextKeyUserId).(model.UserId)
	return userID, ok
}
