// Package user provides user authentication provider interfaces.
package user

import (
	"net/http"
)

type AuthProvider interface {
	// GetSessionKey returns the session key for the user with the given ID.
	GetSessionKey(w http.ResponseWriter, r *http.Request)
}
