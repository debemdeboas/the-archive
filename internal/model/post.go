// Package model defines core data structures and types for the blog application.
package model

import (
	"html/template"
	"strings"
	"time"

	"github.com/debemdeboas/the-archive/internal/util"
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

	Markdown     []byte
	CreatedDate  time.Time
	ModifiedDate time.Time

	// Optional data from Mmark front matter.
	Info *util.ExtendedTitleData

	// Optional data: owner of the post (for example, the user who created it).
	Owner UserID
}

func (p *Post) GetTitle() string {
	if p.Info != nil && p.Info.Title != "" {
		var s strings.Builder

		if p.Info.SeriesInfo.Name != "" && p.Info.SeriesInfo.Value != "" {
			s.WriteString("[")
			s.WriteString(p.Info.SeriesInfo.Name)
			s.WriteString("-")
			s.WriteString(p.Info.SeriesInfo.Value)
			s.WriteString("] ")
		}

		s.WriteString(p.Info.Title)

		return s.String()
	}
	return p.Title
}
