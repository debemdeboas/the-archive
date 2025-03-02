package auth

import (
	"embed"
	"html/template"
	"log"
	"net/http"

	"github.com/debemdeboas/the-archive/internal/config"
)

// RegisterEd25519AuthRoutes registers all the routes needed for RSA authentication
func RegisterEd25519AuthRoutes(mux *http.ServeMux, provider *Ed25519AuthProvider, fs *embed.FS) {
	tmpl, err := template.ParseFS(
		fs,
		config.TemplatesLocalDir+"/"+config.TemplateLayout,
		config.TemplatesLocalDir+"/ed25519_auth.html",
	)
	if err != nil {
		log.Fatal("Error loading auth template:", err)
	}

	mux.HandleFunc("/auth/challenge", Ed25519ChallengeHandler(provider))
	mux.HandleFunc("/auth/verify", Ed25519VerifyHandler(provider))
	mux.HandleFunc("/auth/login", Ed25519AuthPageHandler(provider, tmpl))
}
