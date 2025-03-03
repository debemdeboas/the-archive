package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/debemdeboas/the-archive/internal/config"
	"github.com/debemdeboas/the-archive/internal/model"
	"github.com/rs/zerolog"
)

// Ed25519AuthProvider implements AuthProvider with Ed25519-based auth
type Ed25519AuthProvider struct {
	publicKey  ed25519.PublicKey
	headerName string
	cookieName string
	userId     model.UserId
	challenge  []byte
}

// NewEd25519AuthProvider creates a new Ed25519-based auth provider
func NewEd25519AuthProvider(publicKeyPEM string, headerName string, userId model.UserId) (*Ed25519AuthProvider, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	publicKey, ok := pub.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("key is not an Ed25519 public key")
	}

	challenge := make([]byte, 32)
	if _, err := rand.Read(challenge); err != nil {
		return nil, fmt.Errorf("failed to generate challenge: %w", err)
	}

	return &Ed25519AuthProvider{
		publicKey:  publicKey,
		headerName: headerName,
		cookieName: "auth_token",
		userId:     userId,
		challenge:  challenge,
	}, nil
}

// WithHeaderAuthorization returns middleware that validates Ed25519-signed messages
func (p *Ed25519AuthProvider) WithHeaderAuthorization() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			l := zerolog.Ctx(r.Context())

			// Try to get signature from header or cookie
			var signature []byte
			var err error

			// Try header first
			authHeader := r.Header.Get(p.headerName)
			if authHeader != "" {
				signature, err = base64.StdEncoding.DecodeString(strings.TrimSpace(authHeader))
				if err != nil {
					l.Error().Err(err).Msg("Failed to decode signature from header")
				}
			}

			// If header auth failed, try cookie
			if len(signature) == 0 {
				cookie, err := r.Cookie(p.cookieName)
				if err == nil && cookie.Value != "" {
					signature, err = base64.StdEncoding.DecodeString(cookie.Value)
					if err != nil {
						l.Error().Err(err).Msg("Failed to decode signature from cookie")
					}
				}
			}

			// If we have a signature, verify it
			if len(signature) > 0 {
				if ed25519.Verify(p.publicKey, p.challenge, signature) {
					// Signature valid, set user ID in context and proceed
					ctx := r.Context()
					ctx = ContextWithUserId(ctx, p.userId)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// No valid signature (or none provided), proceed without user ID
			next.ServeHTTP(w, r)
		})
	}
}

// GetUserIdFromSession extracts the user ID from the request
func (p *Ed25519AuthProvider) GetUserIdFromSession(r *http.Request) (model.UserId, error) {
	l := zerolog.Ctx(r.Context())
	userId := r.Context().Value(ContextKeyUserId)
	if userId == nil {
		l.Warn().Msg("No user ID found in context")
		return "", errors.New("no user ID in context")
	}
	return userId.(model.UserId), nil
}

// HandleWebhookUser is a no-op for this simple provider
func (p *Ed25519AuthProvider) HandleWebhookUser(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// GetChallenge returns the current challenge that needs to be signed
func (p *Ed25519AuthProvider) GetChallenge() []byte {
	return p.challenge
}

// RefreshChallenge generates a new random challenge
func (p *Ed25519AuthProvider) RefreshChallenge() error {
	challenge := make([]byte, 32)
	if _, err := rand.Read(challenge); err != nil {
		authLogger.Error().Err(err).Msg("Failed to generate challenge")
		return fmt.Errorf("failed to generate challenge: %w", err)
	}
	p.challenge = challenge
	return nil
}

// EnforceUserAndGetId enforces the user and returns the user ID
func (p *Ed25519AuthProvider) EnforceUserAndGetId(w http.ResponseWriter, r *http.Request) (model.UserId, error) {
	l := zerolog.Ctx(r.Context())
	userId, err := p.GetUserIdFromSession(r)
	if err != nil {
		l.Warn().Err(err).Msg("Unauthorized access attempt")

		// Set Hx-Redirect to auth page
		w.Header().Add(config.HHxRedirect, "/auth/login")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return "", err
	}

	return userId, nil
}
