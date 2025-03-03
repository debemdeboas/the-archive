package repository

import (
	"github.com/debemdeboas/the-archive/internal/model"
	"github.com/rs/zerolog"
)

type PostRepository interface {
	Init()
	GetPosts() ([]model.Post, map[string]*model.Post, error)
	GetPostList() []model.Post
	ReadPost(id any) (*model.Post, error)
	ReloadPosts()

	NewPost() *model.Post
	SavePost(post *model.Post) error
	SetPostContent(post *model.Post) error

	// SetReloadNotifier sets a function that will be called when the posts are reloaded.
	SetReloadNotifier(notifier func(model.PostId))
}

var repoLogger zerolog.Logger

func SetLogger(logger zerolog.Logger) {
	repoLogger = logger
}
