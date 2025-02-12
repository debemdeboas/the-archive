package model

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/debemdeboas/the-archive/internal/config"
	"github.com/debemdeboas/the-archive/internal/theme"
)

type PageData struct {
	SiteName string

	PageURL string

	Theme string

	SyntaxCSS    template.CSS
	SyntaxTheme  string
	SyntaxThemes []string

	ShowToolbar  *bool
	IsEditorPage *bool
}

func NewPageData(r *http.Request) *PageData {
	syntaxtheme := theme.GetSyntaxThemeFromRequest(r)
	return &PageData{
		SiteName:     config.SiteName,
		PageURL:      r.URL.Path,
		Theme:        theme.GetThemeFromRequest(r),
		SyntaxTheme:  syntaxtheme,
		SyntaxThemes: theme.GetSyntaxThemes(),
		SyntaxCSS:    theme.GenerateSyntaxCSS(syntaxtheme),
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
