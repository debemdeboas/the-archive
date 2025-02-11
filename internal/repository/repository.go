package repository

import "github.com/debemdeboas/the-archive/internal/model"

type PostRepository interface {
	Init()
	GetPosts() ([]model.Post, map[string]*model.Post, error)
	GetPostList() []model.Post
	ReadPost(path any) ([]byte, error)
	ReloadPosts()

	// SetReloadNotifier sets a function that will be called when the posts are reloaded.
	SetReloadNotifier(notifier func(model.PostID))
}
