package auth

import (
	"embed"
	"html/template"
	"net/http"

	"github.com/debemdeboas/the-archive/internal/config"
	"github.com/debemdeboas/the-archive/internal/routes"
)

// RegisterEd25519AuthRoutes registers all the routes needed for RSA authentication
func RegisterEd25519AuthRoutes(mux *http.ServeMux, provider *Ed25519AuthProvider, fs *embed.FS) {
	tmpl, err := template.ParseFS(
		fs,
		config.TemplatesLocalDir+"/"+config.TemplateLayout,
		config.TemplatesLocalDir+"/ed25519_auth.html",
	)
	if err != nil {
		authLogger.Fatal().Err(err).Msg("Error loading auth template")
		return
	}

	mux.HandleFunc(routes.AuthChallenge, Ed25519ChallengeHandler(provider))
	mux.HandleFunc(routes.AuthVerify, Ed25519VerifyHandler(provider))
	mux.HandleFunc(routes.AuthLogin, Ed25519AuthPageHandler(provider, tmpl))
}
