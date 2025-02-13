package editor

import "github.com/debemdeboas/the-archive/internal/model"

type DraftId model.PostId

type Draft struct {
	Id      DraftId
	Content []byte
}

type Repository interface {
	CreateDraft() (*Draft, error)
	SaveDraft(id DraftId, content []byte) error
	GetDraft(id DraftId) (*Draft, error)
	DeleteDraft(id DraftId) error
}
