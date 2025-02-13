package editor

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
)

type MemoryRepository struct {
	drafts sync.Map
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{}
}

func (m *MemoryRepository) CreateDraft() (*Draft, error) {
	id := DraftId(uuid.New().String())
	draft := &Draft{
		Id:          id,
		Content:     []byte{},
		Initialized: false,
	}
	m.drafts.Store(id, draft)
	return draft, nil
}

func (r *MemoryRepository) SaveDraft(id DraftId, content []byte) error {
	if draft, ok := r.drafts.Load(id); ok {
		d := draft.(*Draft)
		d.Initialized = len(content) > 0
		d.Content = content
	} else {
		if len(content) == 0 {
			return nil
		}

		r.drafts.Store(id, &Draft{
			Id:      id,
			Content: content,
		})
	}

	return nil
}

func (m *MemoryRepository) GetDraft(id DraftId) (*Draft, error) {
	if draft, ok := m.drafts.Load(id); ok {
		return draft.(*Draft), nil
	}
	return nil, fmt.Errorf("draft not found: %s", id)
}

func (m *MemoryRepository) DeleteDraft(id DraftId) error {
	m.drafts.Delete(id)
	return nil
}
