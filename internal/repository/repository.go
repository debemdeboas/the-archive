// Package repository provides post storage and retrieval interfaces and implementations.
package repository

import (
	"time"

	"github.com/debemdeboas/the-archive/internal/model"
	"github.com/rs/zerolog"
)

type PostRepository interface {
	Init()
	GetPosts() ([]model.Post, map[string]*model.Post, error)
	GetPostList() []model.Post
	ReadPost(id any) (*model.Post, error)
	GetAdjacentPosts(id any) (prev *model.Post, next *model.Post)
	ReloadPosts()

	NewPost() *model.Post
	SavePost(post *model.Post) error
	SetPostContent(post *model.Post) error

	// SetReloadNotifier sets a function that will be called when the posts are reloaded.
	SetReloadNotifier(notifier func(model.PostID))

	// SetReloadTimeout sets the timeout for reloading posts.
	SetReloadTimeout(timeout time.Duration)
}

var repoLogger zerolog.Logger

func SetLogger(logger zerolog.Logger) {
	repoLogger = logger
}
