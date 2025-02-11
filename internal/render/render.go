package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/debemdeboas/the-archive/internal/theme"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

func HighlightCode(code, language, highlightTheme string) string {
	lexer := lexers.Get(language)
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return code
	}

	var buf strings.Builder
	style := styles.Get(highlightTheme)
	formatter := theme.GetFormatter()
	err = formatter.Format(&buf, style, iterator)
	if err != nil {
		return code
	}

	return buf.String()
}

func RenderMarkdown(md []byte, highlightTheme string) []byte {
	opts := html.RendererOptions{
		Flags:    html.CommonFlags | html.HrefTargetBlank | html.FootnoteReturnLinks,
		Comments: [][]byte{[]byte("//"), []byte("#")},
		RenderNodeHook: func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
			if code, ok := node.(*ast.CodeBlock); ok && entering {
				var lang string
				if info := code.Info; info != nil {
					lang = string(info)
				}
				highlighted := HighlightCode(string(code.Literal), lang, highlightTheme)
				fmt.Fprintf(w, "<div class=\"highlight\">%s</div>", highlighted)
				return ast.GoToNext, true
			}

			return ast.GoToNext, false
		},
	}

	doc := parser.NewWithExtensions(
		parser.CommonExtensions |
			parser.AutoHeadingIDs |
			parser.Footnotes |
			parser.SuperSubscript |
			parser.Mmark,
	).Parse(md)
	rendered := markdown.Render(doc, html.NewRenderer(opts))

	return rendered
}
