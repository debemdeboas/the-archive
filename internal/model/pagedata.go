package model

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/debemdeboas/the-archive/internal/config"
	"github.com/debemdeboas/the-archive/internal/theme"
)

type PageData struct {
	SiteName    string
	SiteTagline string
	SiteDescription string
	SiteKeywords []string
	SiteAuthor   string

	PageURL string

	Theme           string
	AllowThemeSwitching bool
	EditorEnabled       bool
	LivePreviewEnabled  bool

	SyntaxCSS    template.CSS
	SyntaxTheme  string
	SyntaxThemes []string

	ShowToolbar  *bool
	IsEditorPage *bool
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
		LivePreviewEnabled:  config.AppConfig.Features.Editor.LivePreview,
		SyntaxTheme:         syntaxtheme,
		SyntaxThemes:        theme.GetSyntaxThemes(),
		SyntaxCSS:           theme.GenerateSyntaxCSS(syntaxtheme),
	}
}

func (pd *PageData) IsPost() bool {
	if pd.ShowToolbar == nil {
		return strings.HasPrefix(pd.PageURL, config.PostsUrlPath)
	}
	return *pd.ShowToolbar
}

func (pd *PageData) IsEditor() bool {
	if pd.IsEditorPage == nil {
		return strings.HasPrefix(pd.PageURL, "/new/post/edit")
	}
	return *pd.IsEditorPage
}
