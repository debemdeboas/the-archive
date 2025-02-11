package model

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/debemdeboas/the-archive/internal/config"
	"github.com/debemdeboas/the-archive/internal/theme"
)

type PageData struct {
	PageURL string

	Theme string

	SyntaxCSS    template.CSS
	SyntaxTheme  string
	SyntaxThemes []string
}

func NewPageData(r *http.Request) *PageData {
	syntaxtheme := theme.GetSyntaxThemeFromRequest(r)
	return &PageData{
		PageURL:      r.URL.Path,
		Theme:        theme.GetThemeFromRequest(r),
		SyntaxTheme:  syntaxtheme,
		SyntaxThemes: theme.GetSyntaxThemes(),
		SyntaxCSS:    theme.GenerateSyntaxCSS(syntaxtheme),
	}
}

func (pd *PageData) IsPost() bool {
	return strings.HasPrefix(pd.PageURL, config.PostsUrlPath)
}
