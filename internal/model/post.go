package model

import (
	"html/template"
	"time"
)

type PostID string

type Post struct {
	ID PostID

	Title   string
	Content template.HTML
	Path    string

	// Used for cache busting.
	// We cannot use the content hash because the content is already rendered.
	MDContentHash string

	ModifiedDate time.Time
}
