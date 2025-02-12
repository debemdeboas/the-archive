package editor

// type EditorRepository interface {
// 	SaveDraft(id string, content []byte) error
// 	GetDraft(id string) ([]byte, error)
// }

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
