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
	id := DraftID(uuid.New().String())
	draft := &Draft{
		ID:          id,
		Content:     []byte{},
		Initialized: false,
	}
	m.drafts.Store(id, draft)
	return draft, nil
}

func (m *MemoryRepository) SaveDraft(id DraftID, content []byte) error {
	if draft, ok := m.drafts.Load(id); ok {
		d := draft.(*Draft)
		d.Initialized = len(content) > 0
		d.Content = content
	} else {
		if len(content) == 0 {
			return nil
		}

		m.drafts.Store(id, &Draft{
			ID:      id,
			Content: content,
		})
	}

	return nil
}

func (m *MemoryRepository) GetDraft(id DraftID) (*Draft, error) {
	if draft, ok := m.drafts.Load(id); ok {
		return draft.(*Draft), nil
	}
	return nil, fmt.Errorf("draft not found: %s", id)
}

func (m *MemoryRepository) DeleteDraft(id DraftID) error {
	m.drafts.Delete(id)
	return nil
}
