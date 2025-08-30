// Package util provides utility functions for content hashing and front matter parsing.
package util

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"

	"github.com/BurntSushi/toml"
	"github.com/gomarkdown/markdown"

	"github.com/mmarkdown/mmark/v2/mast"
)

type ExtendedTitleData struct {
	*mast.TitleData
	Consumed     int
	ToolbarTitle string
}

func ContentHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

func ContentHashString(content string) string {
	return ContentHash([]byte(content))
}

func GetFrontMatter(md []byte) *ExtendedTitleData {
	md = markdown.NormalizeNewlines(md)
	md = bytes.TrimLeft(md, "\n \t\r")

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

	end := second + 2*len(delimiter) + 1
	if end > len(md) {
		return nil
	}

	frontMatter := md[3 : end-4]
	info := &ExtendedTitleData{
		TitleData: &mast.TitleData{},
	}

	if _, err := toml.Decode(string(frontMatter), info); err != nil {
		return nil
	}

	if info.Language == "" {
		info.Language = "en"
	}
	info.Consumed = end

	return info
}
