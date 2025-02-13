package editor

type FSEditorRepository struct {
}

func NewFSEditorRepository() *FSEditorRepository {
	return &FSEditorRepository{}
}

func (r *FSEditorRepository) SaveDraft(id string, content []byte) error {
	return nil
}

func (r *FSEditorRepository) GetDraft(id string) ([]byte, error) {
	return nil, nil
}

func (r *FSEditorRepository) DeleteDraft(id string) error {
	return nil
}
