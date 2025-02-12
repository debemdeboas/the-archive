package editor

type EditorRepository interface {
	SaveDraft(id string, content []byte) error
	GetDraft(id string) ([]byte, error)
}
