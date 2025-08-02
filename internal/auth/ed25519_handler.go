package auth

import (
	"crypto/ed25519" // Add this import for Ed25519 verification
	"encoding/base64"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	"github.com/debemdeboas/the-archive/internal/config"
	"github.com/rs/zerolog"
)

// Ed25519ChallengeHandler creates an HTTP handler that serves the current challenge
func Ed25519ChallengeHandler(provider *Ed25519AuthProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := zerolog.Ctx(r.Context())
		switch r.Method {
		case http.MethodGet:
			// Return the current challenge for signing
			challenge := provider.GetChallenge()
			response := map[string]string{
				"challenge": base64.StdEncoding.EncodeToString(challenge),
			}

			w.Header().Set(config.HCType, config.CTypeJSON)
			json.NewEncoder(w).Encode(response)

		case http.MethodPost:
			// Generate a new challenge
			if err := provider.RefreshChallenge(); err != nil {
				l.Error().Err(err).Msg("Failed to refresh challenge")
				http.Error(w, config.ErrRefreshChallengeFmt, http.StatusInternalServerError)
				return
			}

			challenge := provider.GetChallenge()
			response := map[string]string{
				"challenge": base64.StdEncoding.EncodeToString(challenge),
			}

			w.Header().Set(config.HCType, config.CTypeJSON)
			json.NewEncoder(w).Encode(response)

		default:
			http.Error(w, config.HTTPErrMethodNotAllowed, http.StatusMethodNotAllowed)
		}
	}
}

// Ed25519VerifyHandler creates an HTTP handler that verifies the signature
func Ed25519VerifyHandler(provider *Ed25519AuthProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, config.HTTPErrMethodNotAllowed, http.StatusMethodNotAllowed)
			return
		}

		// Get authorization header
		authHeader := r.Header.Get(provider.headerName)
		if authHeader == "" {
			http.Error(w, config.ErrAuthHeaderRequired, http.StatusUnauthorized)
			return
		}

		signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(authHeader))
		if err != nil {
			authLogger.Error().Err(err).Msg("Failed to decode signature")
			http.Error(w, config.ErrInvalidSignatureFormat, http.StatusUnauthorized)
			return
		}

		// Verify the signature against the challenge
		if !ed25519.Verify(provider.publicKey, provider.challenge, signature) {
			authLogger.Error().
				Str("signature", string(signature)).
				Str("challenge", string(provider.challenge)).
				Msg("Signature verification failed")
			http.Error(w, config.ErrInvalidSignature, http.StatusUnauthorized)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     config.CookieAuthToken,
			Value:    base64.StdEncoding.EncodeToString(signature),
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
			Secure:   r.TLS != nil,
			MaxAge:   3600 * 24, // 24 hours
		})

		w.WriteHeader(http.StatusOK)
	}
}

// Ed25519AuthPageHandler serves the authentication page
func Ed25519AuthPageHandler(provider *Ed25519AuthProvider, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := zerolog.Ctx(r.Context())
		redirectURL := r.URL.Query().Get("redirect")
		if redirectURL == "" {
			redirectURL = "/"
		}

		data := struct {
			RedirectURL string
		}{
			RedirectURL: redirectURL,
		}

		w.Header().Set(config.HCType, config.CTypeHTML)
		w.Header().Add(config.HHxRedirect, redirectURL)

		shouldRefresh := r.URL.Query().Get("refresh")
		if shouldRefresh != "" {
			bShouldRefresh := shouldRefresh == "true"
			if bShouldRefresh { // Refresh the page
				w.Header().Set(config.HHxRedirect, "/auth/login")
			}
		}

		err := tmpl.ExecuteTemplate(w, config.TemplateNameAuth, data)
		if err != nil {
			l.Error().Err(err).Msg("Failed to render auth template")
			http.Error(w, config.ErrInternalServerError, http.StatusInternalServerError)
		}
	}
}
