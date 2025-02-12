package editor

import (
	"errors"
	"sync"
)

type MemoryEditorRepository struct {
	drafts sync.Map
}

func (r *MemoryEditorRepository) SaveDraft(id string, content []byte) error {
	r.drafts.Store(id, content)
	return nil
}

func (r *MemoryEditorRepository) GetDraft(id string) ([]byte, error) {
	if content, ok := r.drafts.Load(id); ok {
		return content.([]byte), nil
	}
	return nil, errors.New("draft not found")
}
