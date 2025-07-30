package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
	clerkuser "github.com/clerk/clerk-sdk-go/v2/user"
	"github.com/debemdeboas/the-archive/internal/db"
	"github.com/debemdeboas/the-archive/internal/model"
	"github.com/rs/zerolog"
)

type ClerkAuthProvider struct {
	db db.Db

	cookieExtractor clerkhttp.AuthorizationOption
}

func NewClerkAuthProvider(clerkKey string) *ClerkAuthProvider {
	clerk.SetKey(clerkKey)

	return &ClerkAuthProvider{
		cookieExtractor: clerkhttp.AuthorizationJWTExtractor(func(r *http.Request) string {
			cookie, err := r.Cookie("__session")
			if err != nil || cookie == nil {
				// Retrieve logger from context if request is available
				if r != nil {
					l := zerolog.Ctx(r.Context())
					l.Error().Err(err).Msg("Authorization cookie not found")
				}
				return ""
			}
			return cookie.Value
		}),
	}
}

func (c *ClerkAuthProvider) WithHeaderAuthorization() func(http.Handler) http.Handler {
	return clerkhttp.WithHeaderAuthorization(c.cookieExtractor)
}

func (c *ClerkAuthProvider) GetUserIdFromSession(r *http.Request) (model.UserId, error) {
	l := zerolog.Ctx(r.Context())
	claims, ok := clerk.SessionClaimsFromContext(r.Context())
	if !ok {
		l.Warn().Msg("Failed to get session claims from context")
		return "", errors.New("failed to get session claims from context")
	}

	usr, err := clerkuser.Get(r.Context(), claims.Subject)
	if err != nil {
		l.Error().Err(err).Msg("Failed to get user from Clerk")
		return "", err
	}

	return model.UserId(usr.ID), nil
}

func (c *ClerkAuthProvider) HandleWebhookUser(w http.ResponseWriter, r *http.Request) {
	l := zerolog.Ctx(r.Context())
	l.Info().Msg("User webhook received")

	type EventPayload struct {
		Data struct {
			clerk.User
		} `json:"data"`

		Type string `json:"type"`
	}

	var usr clerk.User
	var payload EventPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		l.Error().Err(err).Msg("Error decoding event payload")
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	switch payload.Type {
	case "user.created":
		l.Info().Msg("User created webhook received")

		usr = payload.Data.User

		// Need to get Twitter/X username first and error out if it's not available
		if len(usr.ExternalAccounts) == 0 {
			l.Error().Str("user_id", usr.ID).Msg("No external accounts found for user")
			http.Error(w, "No external accounts found", http.StatusBadRequest)
			return
		}

		// Verify if provider is "oauth_x"
		if !strings.EqualFold(usr.ExternalAccounts[0].Provider, "oauth_x") {
			l.Error().Str("user_id", usr.ID).Msg("Invalid provider for user")
			http.Error(w, "Invalid provider", http.StatusBadRequest)
			return
		}

		_, err := c.db.Exec("INSERT INTO users (id, username) VALUES (?, ?)", usr.ID, usr.ExternalAccounts[0].Username)
		if err != nil {
			l.Error().Err(err).Str("user_id", usr.ID).Msg("Error inserting user")
			http.Error(w, "Error saving user", http.StatusInternalServerError)
			return
		}

		l.Info().Str("user_id", usr.ID).Msg("User created")
		w.WriteHeader(http.StatusCreated)

	case "user.updated":
		l.Info().Msg("User updated webhook received")
		w.WriteHeader(http.StatusNoContent)

	case "user.deleted":
		l.Info().Msg("User deleted webhook received")

		usr = payload.Data.User

		_, err := c.db.Exec("DELETE FROM users WHERE id = ?", usr.ID)
		if err != nil {
			l.Error().Err(err).Str("user_id", usr.ID).Msg("Error deleting user")
			http.Error(w, "Error deleting user", http.StatusInternalServerError)
			return
		}

		l.Info().Str("user_id", usr.ID).Msg("User deleted")
		w.WriteHeader(http.StatusNoContent)

	default:
		l.Warn().Str("event_type", payload.Type).Msg("Invalid event type")
		http.Error(w, "Invalid event type", http.StatusBadRequest)
		return
	}
}

func (c *ClerkAuthProvider) EnforceUserAndGetId(w http.ResponseWriter, r *http.Request) (model.UserId, error) {
	l := zerolog.Ctx(r.Context())
	userID, err := c.GetUserIdFromSession(r)
	if err != nil {
		l.Warn().Err(err).Msg("Unauthorized access attempt")
		w.Header().Add("HHxRedirect", "/auth/login") // Assuming this header is defined elsewhere
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return "", err
	}
	return userID, nil
}
