package auth

import (
	"net/http"

	"github.com/debemdeboas/the-archive/internal/model"
)

type AuthProvider interface {
	WithHeaderAuthorization() func(http.Handler) http.Handler

	GetUserIdFromSession(r *http.Request) (model.UserId, error)

	EnforceUserAndGetId(w http.ResponseWriter, r *http.Request) (model.UserId, error)

	HandleWebhookUser(w http.ResponseWriter, r *http.Request)
}
