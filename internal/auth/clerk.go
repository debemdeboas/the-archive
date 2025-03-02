package auth

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
	clerkuser "github.com/clerk/clerk-sdk-go/v2/user"
	"github.com/debemdeboas/the-archive/internal/db"
	"github.com/debemdeboas/the-archive/internal/model"
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
				log.Println("Authorization cookie not found:", err)
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
	claims, ok := clerk.SessionClaimsFromContext(r.Context())
	if !ok {
		return "", errors.New("failed to get session claims from context")
	}

	usr, err := clerkuser.Get(r.Context(), claims.Subject)
	if err != nil {
		return "", err
	}

	return model.UserId(usr.ID), nil
}

func (c *ClerkAuthProvider) HandleWebhookUser(w http.ResponseWriter, r *http.Request) {
	log.Println("User webhook received")

	type EventPayload struct {
		Data struct {
			clerk.User
		} `json:"data"`

		Type string `json:"type"`
	}

	var usr clerk.User
	var payload EventPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Println("Error decoding event payload:", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	switch payload.Type {
	case "user.created":
		log.Println("User created webhook received")

		usr = payload.Data.User

		// Need to get Twitter/X username first and error out if it's not available
		if len(usr.ExternalAccounts) == 0 {
			log.Println("No external accounts found for user", usr.ID)
			http.Error(w, "No external accounts found", http.StatusBadRequest)
			return
		}

		// Verify if provider is "oauth_x"
		if !strings.EqualFold(usr.ExternalAccounts[0].Provider, "oauth_x") {
			log.Println("Invalid provider for user", usr.ID)
			http.Error(w, "Invalid provider", http.StatusBadRequest)
			return
		}

		_, err := c.db.Exec("INSERT INTO users (id, username) VALUES (?, ?)", usr.ID, usr.ExternalAccounts[0].Username)
		if err != nil {
			log.Println("Error inserting user:", err)
			http.Error(w, "Error saving user", http.StatusInternalServerError)
			return
		}

		log.Printf("User %s created", usr.ID)

		w.WriteHeader(http.StatusCreated)
	case "user.updated":

		log.Println("User updated webhook received")
		w.WriteHeader(http.StatusNoContent)

	case "user.deleted":
		log.Println("User deleted webhook received")

		usr = payload.Data.User

		_, err := c.db.Exec("DELETE FROM users WHERE id = ?", usr.ID)
		if err != nil {
			log.Println("Error deleting user:", err)
			http.Error(w, "Error deleting user", http.StatusInternalServerError)
			return
		}

		log.Printf("User %s deleted", usr.ID)

		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Invalid event type", http.StatusBadRequest)
		return
	}
}

func (c *ClerkAuthProvider) EnforceUserAndGetId(w http.ResponseWriter, r *http.Request) (model.UserId, error) {
	return "", nil
}
