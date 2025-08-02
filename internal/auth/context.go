package auth

import (
	"context"

	"github.com/debemdeboas/the-archive/internal/model"
)

// ContextKey is a type for context keys to avoid collisions
type ContextKey string

// ContextKeyUserID is the key for user ID in request context
const ContextKeyUserID ContextKey = "userID"

// ContextWithUserID returns a new context with the user ID set
func ContextWithUserID(ctx context.Context, userID model.UserID) context.Context {
	return context.WithValue(ctx, ContextKeyUserID, userID)
}

// UserIDFromContext extracts the user ID from context
func UserIDFromContext(ctx context.Context) (model.UserID, bool) {
	userID, ok := ctx.Value(ContextKeyUserID).(model.UserID)
	return userID, ok
}
