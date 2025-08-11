package auth

import (
	"net/http"

	"github.com/debemdeboas/the-archive/internal/model"
)

type AuthProvider interface {
	WithHeaderAuthorization() func(http.Handler) http.Handler

	GetUserIDFromSession(r *http.Request) (model.UserID, error)

	EnforceUserAndGetID(w http.ResponseWriter, r *http.Request) (model.UserID, error)

	HandleWebhookUser(w http.ResponseWriter, r *http.Request)
}
