package cache

import "html/template"

var syntaxCache = NewCache[string, template.CSS]()

func GetSyntaxCSS(theme string) (template.CSS, bool) {
	return syntaxCache.Get(theme)
}

func SetSyntaxCSS(theme string, css template.CSS) {
	syntaxCache.Set(theme, css)
}
