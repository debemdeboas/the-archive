package config

import "regexp"

const (
	MarkdownRenderer = "mmark"
)

var (
	RegexCallout = regexp.MustCompile(`//\s*<<(\d+)>>`)
)
