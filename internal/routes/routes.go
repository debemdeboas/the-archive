// Package routes defines HTTP route constants for the application.
package routes

// API Routes
const (
	// Static and assets
	RobotsPath         = "/robots.txt"
	ThemeOppositeIcon  = "/theme/opposite-icon"
	PartialsPost       = "/partials/post"
	ThemeToggle        = "/theme/toggle"
	SyntaxThemeSet     = "/syntax-theme/set"
	SyntaxThemeGet     = "/syntax-theme/{theme}"
	
	// SSE
	SSEPath = "/sse"
	
	// Root
	RootPath = "/"
	
	// Editor routes
	NewPost             = "/new/post"
	NewPostEdit         = "/new/post/edit"
	EditPost            = "/edit/post/"
	PartialsPostPreview = "/partials/post/preview"
	PartialsDraftPreview = "/partials/draft/preview"
	
	// API
	APIPosts = "/api/posts/{id}"
	APIImages = "/api/images"
	
	// Auth routes
	AuthChallenge = "/auth/challenge"
	AuthVerify    = "/auth/verify"
	AuthLogin     = "/auth/login"
)