package model

import (
	"html/template"
	"strings"
	"time"

	"github.com/mmarkdown/mmark/v2/mast"
)

type PostId string

type Post struct {
	Id PostId

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
	Info *mast.TitleData

	// Optional data: owner of the post (for example, the user who created it).
	Owner UserId
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
