package util

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/parser"

	"github.com/mmarkdown/mmark/v2/mast"
	"github.com/mmarkdown/mmark/v2/mparser"
)

func ContentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

func ContentHashString(content string) string {
	return ContentHash([]byte(content))
}

func GetFrontMatter(md []byte) *mast.TitleData {
	md = markdown.NormalizeNewlines(md)

	delimiter := []byte("%%%")

	// Check if md is long enough to contain the delimiter
	if len(md) < 2*len(delimiter) {
		return nil
	}

	first := bytes.Index(md[:len(delimiter)+1], delimiter)
	if first == -1 {
		return nil
	}

	second := bytes.Index(md[first+len(delimiter):], delimiter)
	if second == -1 {
		return nil
	}

	frontMatter := md[:second+2*len(delimiter)+1]
	var info *mast.TitleData

	p := parser.NewWithExtensions(mparser.Extensions)
	p.Opts = parser.Options{
		ParserHook: func(data []byte) (ast.Node, []byte, int) {
			node, data, consumed := mparser.Hook(data)
			if t, ok := node.(*mast.Title); ok {
				info = t.TitleData
			}
			return node, data, consumed
		},
		Flags: parser.FlagsNone,
	}

	_ = markdown.Parse(frontMatter, p)

	return info
}
