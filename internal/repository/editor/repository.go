// Package editor provides draft management interfaces and data structures for the blog editor.
package editor

import "github.com/debemdeboas/the-archive/internal/model"

type DraftID model.PostID

type Draft struct {
	ID      DraftID
	Content []byte

	Initialized bool
}

type Repository interface {
	CreateDraft() (*Draft, error)
	SaveDraft(id DraftID, content []byte) error
	GetDraft(id DraftID) (*Draft, error)
	DeleteDraft(id DraftID) error
}
