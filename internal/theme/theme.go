// Package theme handles theme management, syntax highlighting, and CSS generation.
package theme

import (
	"html/template"
	"net/http"
	"slices"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/debemdeboas/the-archive/internal/cache"
	"github.com/debemdeboas/the-archive/internal/config"
)

func GetThemeFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie(config.CookieTheme); err == nil {
		return cookie.Value
	}
	return config.AppConfig.Theme.Default
}

func GetDefaultSyntaxTheme(theme string) string {
	return map[string]string{
		config.LightTheme: config.AppConfig.Theme.SyntaxHighlighting.DefaultLight,
		config.DarkTheme:  config.AppConfig.Theme.SyntaxHighlighting.DefaultDark,
	}[theme]
}

func GetSyntaxThemeFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie(config.CookieSyntaxTheme); err == nil {
		return cookie.Value
	}
	return GetDefaultSyntaxTheme(GetThemeFromRequest(r))
}

func GetSyntaxThemes() []string {
	styleNames := styles.Names()
	slices.Sort(styleNames)
	return styleNames
}

func GetFormatter() *html.Formatter {
	formatter := html.New(
		html.WithClasses(true),
		html.TabWidth(4),
		html.WithLineNumbers(true),
		html.WrapLongLines(true),
	)
	return formatter
}

func GenerateSyntaxCSS(theme string) template.CSS {
	if css, ok := cache.GetSyntaxCSS(theme); ok {
		return css
	}

	var buf strings.Builder
	formatter := GetFormatter()
	style := styles.Get(theme)

	bg := style.Get(chroma.Background)
	if !bg.Colour.IsSet() {
		// Calculate the color of highlighted text given the background color
		// for when the Chroma theme doesn't supply a default
		luminance := (0.299*float64(bg.Background.Red()) +
			0.587*float64(bg.Background.Green()) +
			0.114*float64(bg.Background.Blue())) / 255
		if luminance > 0.5 {
			buf.WriteString(".chroma { color: #181818; }\n")
		}
	}

	formatter.WriteCSS(&buf, style)
	css := template.CSS(buf.String())
	cache.SetSyntaxCSS(theme, css)
	return css
}

func GetThemeIcon(theme string) string {
	if theme == config.LightTheme {
		return config.DarkThemeIcon
	}
	return config.LightThemeIcon
}
