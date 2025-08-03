// Package render provides markdown rendering and syntax highlighting functionality.
package render

import (
	"fmt"
	"html"
	"io"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/debemdeboas/the-archive/internal/cache"
	"github.com/debemdeboas/the-archive/internal/config"
	"github.com/debemdeboas/the-archive/internal/theme"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	md_html "github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"

	"github.com/mmarkdown/mmark/v2/lang"
	"github.com/mmarkdown/mmark/v2/mast"
	"github.com/mmarkdown/mmark/v2/mparser"
	"github.com/mmarkdown/mmark/v2/render/mhtml"
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

	res := html.UnescapeString(buf.String())
	res = config.RegexCallout.ReplaceAllString(res, "<span class=\"callout\">$1</span>")
	return res
}

func RenderMarkdown(md []byte, highlightTheme string) ([]byte, any) {
	switch config.MarkdownRenderer {
	case "mmark":
		return RenderMarkdownMmark(md, highlightTheme)
	default:
		return RenderMarkdownClassic(md, highlightTheme), nil
	}
}

// Mutex to protect the check-render-set operation in RenderMarkdownCached
var renderCacheMutex sync.Mutex

func RenderMarkdownCached(md []byte, contentHash, highlightTheme string) ([]byte, any) {
	if contentHash == "" {
		renderLogger.Warn().Msg("Content hash is empty, skipping cache check")
		return RenderMarkdown(md, highlightTheme)
	}

	// First check cache without locking (fast path for cache hits)
	if cached, found := cache.GetRenderedMarkdown(contentHash, highlightTheme); found {
		renderLogger.Debug().Str("contentHash", contentHash).Str("highlightTheme", highlightTheme).Msg("Cache hit for rendered markdown")
		return cached.HTML, cached.Extra
	}

	// Cache miss
	renderLogger.Debug().Str("contentHash", contentHash).Str("highlightTheme", highlightTheme).Msg("Cache miss for rendered markdown")
	renderCacheMutex.Lock()
	defer renderCacheMutex.Unlock()

	html, extra := RenderMarkdown(md, highlightTheme)
	cache.SetRenderedMarkdown(contentHash, highlightTheme, html, extra)

	return html, extra
}

func RenderMarkdownClassic(md []byte, highlightTheme string) []byte {
	opts := md_html.RendererOptions{
		Flags:    md_html.CommonFlags | md_html.HrefTargetBlank | md_html.FootnoteReturnLinks,
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

			if callout, ok := node.(*ast.Callout); ok && entering {
				fmt.Fprintf(w, "<span class=\"callout\">%s</span>", callout.ID)
				return ast.GoToNext, true
			}

			return ast.GoToNext, false
		},
	}

	doc := parser.NewWithExtensions(
		parser.Tables | parser.FencedCode | parser.Autolink | parser.Strikethrough | parser.SpaceHeadings |
			parser.HeadingIDs | parser.BackslashLineBreak | parser.SuperSubscript | parser.DefinitionLists | parser.MathJax |
			parser.AutoHeadingIDs | parser.Footnotes | parser.Strikethrough | parser.OrderedListStart | parser.Attributes |
			parser.Mmark | parser.Includes | parser.NonBlockingSpace,
	).Parse(md)
	rendered := markdown.Render(doc, md_html.NewRenderer(opts))

	return rendered
}

func RenderMarkdownMmark(md []byte, highlightTheme string) ([]byte, *mast.TitleData) {
	md = markdown.NormalizeNewlines(md)

	mparser.Extensions |= parser.NoIntraEmphasis

	p := parser.NewWithExtensions(mparser.Extensions)

	init := mparser.NewInitial("")
	var info *mast.TitleData

	p.Opts = parser.Options{
		ParserHook: func(data []byte) (ast.Node, []byte, int) {
			node, data, consumed := mparser.Hook(data)
			if t, ok := node.(*mast.Title); ok {
				info = t.TitleData
			}
			return node, data, consumed
		},
		ReadIncludeFn: init.ReadInclude,
		Flags:         parser.FlagsNone,
	}

	doc := markdown.Parse(md, p)

	mparser.AddIndex(doc)

	// There's a possibility that info.Language is a nil pointer
	// so we need to check for that before passing it to the lang.New function
	if info == nil {
		info = &mast.TitleData{
			Title:    "Untitled",
			Language: "en",
		}
	}

	mhtmlOpts := mhtml.RendererOptions{
		Language: lang.New(info.Language),
	}

	opts := md_html.RendererOptions{
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

			return mhtmlOpts.RenderHook(w, node, entering)
		},
		Flags: md_html.CommonFlags | md_html.FootnoteNoHRTag | md_html.FootnoteReturnLinks,
	}

	renderer := md_html.NewRenderer(opts)

	x := markdown.Render(doc, renderer)

	return x, info
}

// WarmCache pre-renders markdown content asynchronously to warm the cache
func WarmCache(md []byte, contentHash, highlightTheme string) {
	renderLogger.Debug().Str("contentHash", contentHash).Str("highlightTheme", highlightTheme).Msg("Starting cache warming")
	go func() {
		RenderMarkdownCached(md, contentHash, highlightTheme)
		renderLogger.Debug().Str("contentHash", contentHash).Str("highlightTheme", highlightTheme).Msg("Cache warming completed")
	}()
}
