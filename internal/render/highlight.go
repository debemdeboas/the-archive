package render

import (
	"bytes"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

func HighlightMarkdown(markdown string, theme string) (string, error) {
	lexer := lexers.Get("markdown")
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	style := styles.Get(theme)
	if style == nil {
		style = styles.Fallback
	}

	formatter := html.New(
		html.WithClasses(true),
		html.WithLineNumbers(false),
		html.PreventSurroundingPre(true),
	)

	var buf bytes.Buffer
	iterator, err := lexer.Tokenise(nil, markdown)
	if err != nil {
		return markdown, err
	}

	err = formatter.Format(&buf, style, iterator)
	if err != nil {
		return markdown, err
	}

	// Wrap in a div with markdown-editor class for styling
	result := `<div class="markdown-editor">` + buf.String() + `</div>`

	// Replace newlines with <br> to preserve line breaks
	result = strings.ReplaceAll(result, "\n", "<br>\n")

	return result, nil
}
