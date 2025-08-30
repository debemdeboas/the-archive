package model

import (
	"context"
	"html/template"
	"net/http"
	"strings"

	"github.com/debemdeboas/the-archive/internal/config"
	"github.com/debemdeboas/the-archive/internal/theme"
)

type PageData struct {
	SiteName         string
	SiteTagline      string
	SiteDescription  string
	SiteKeywords     []string
	SiteAuthor       string
	SiteToolbarTitle string

	PageURL string

	Theme               string
	AllowThemeSwitching bool
	EditorEnabled       bool
	DraftsEnabled       bool
	LivePreviewEnabled  bool
	IsAuthenticated     bool

	SyntaxCSS    template.CSS
	SyntaxTheme  string
	SyntaxThemes []string

	ShowToolbar  *bool
	IsEditorPage *bool
}

// Context key for storing authentication status
type contextKey string

const AuthStatusKey contextKey = "authStatus"

// WithAuthStatus adds authentication status to context
func WithAuthStatus(ctx context.Context, isAuthenticated bool) context.Context {
	return context.WithValue(ctx, AuthStatusKey, isAuthenticated)
}

// GetAuthStatus gets authentication status from context
func GetAuthStatus(ctx context.Context) bool {
	if status, ok := ctx.Value(AuthStatusKey).(bool); ok {
		return status
	}
	return false
}

func NewPageData(r *http.Request) *PageData {
	syntaxtheme := theme.GetSyntaxThemeFromRequest(r)

	return &PageData{
		SiteName:            config.AppConfig.Site.Name,
		SiteTagline:         config.AppConfig.Site.Tagline,
		SiteDescription:     config.AppConfig.Site.Description,
		SiteKeywords:        config.AppConfig.Meta.Keywords,
		SiteAuthor:          config.AppConfig.Meta.Author,
		PageURL:             r.URL.Path,
		Theme:               theme.GetThemeFromRequest(r),
		AllowThemeSwitching: config.AppConfig.Theme.AllowSwitching,
		EditorEnabled:       config.AppConfig.Features.Editor.Enabled,
		DraftsEnabled:       config.AppConfig.Features.Editor.Enabled && config.AppConfig.Features.Editor.EnableDrafts,
		LivePreviewEnabled:  config.AppConfig.Features.Editor.LivePreview,
		IsAuthenticated:     GetAuthStatus(r.Context()),
		SyntaxTheme:         syntaxtheme,
		SyntaxThemes:        theme.GetSyntaxThemes(),
		SyntaxCSS:           theme.GenerateSyntaxCSS(syntaxtheme),
	}
}

func (pd *PageData) IsPost() bool {
	if pd.ShowToolbar == nil {
		return strings.HasPrefix(pd.PageURL, config.PostsURLPath)
	}
	return *pd.ShowToolbar
}

func (pd *PageData) IsEditor() bool {
	if pd.IsEditorPage == nil {
		return pd.PageURL == "/new/post/edit" || strings.HasPrefix(pd.PageURL, "/new/post/edit/")
	}
	return *pd.IsEditorPage
}
