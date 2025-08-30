package repository

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/debemdeboas/the-archive/internal/cache"
	"github.com/debemdeboas/the-archive/internal/model"
	"github.com/debemdeboas/the-archive/internal/util"
	"github.com/mmarkdown/mmark/v2/mast"
)

type FSPostRepository struct { // implements PostRepository
	postsPath string

	postsCache       *cache.Cache[string, *model.Post]
	postsCacheSorted []model.Post

	reloadTimeout  time.Duration
	reloadNotifier func(model.PostID)
}

func NewFSPostRepository(postsPath string) *FSPostRepository {
	return &FSPostRepository{
		postsPath:  postsPath,
		postsCache: cache.NewCache[string, *model.Post](),
	}
}

func (r *FSPostRepository) SetReloadNotifier(notifier func(model.PostID)) {
	r.reloadNotifier = notifier
}

func (r *FSPostRepository) notifyPostReload(postID model.PostID) {
	if r.reloadNotifier != nil {
		r.reloadNotifier(postID)
	}
}

func (r *FSPostRepository) Init() {
	posts, postMap, err := r.GetPosts()
	if err != nil {
		repoLogger.Fatal().Err(err).Msg("Error initializing posts")
	}

	r.postsCacheSorted = posts
	r.postsCache.SetTo(postMap)

	go r.ReloadPosts()
}

func (r *FSPostRepository) GetPostList() []model.Post {
	return r.postsCacheSorted
}

func (r *FSPostRepository) GetPosts() ([]model.Post, map[string]*model.Post, error) {
	entries, err := os.ReadDir(r.postsPath)
	if err != nil {
		return nil, nil, err
	}

	var posts []model.Post
	postsMap := make(map[string]*model.Post)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			name := strings.TrimSuffix(entry.Name(), ".md")

			mdContent, err := os.ReadFile(filepath.Join(r.postsPath, name+".md"))
			if err != nil {
				return nil, nil, err
			}

			fileInfo, err := entry.Info()
			if err != nil {
				return nil, nil, err
			}

			info := util.GetFrontMatter(mdContent)
			if info == nil {
				info = &util.ExtendedTitleData{
					TitleData: &mast.TitleData{
						Title: name,
					},
					ToolbarTitle: name,
				}
			}

			post := model.Post{
				ID:            model.PostID(util.ContentHashString(name)),
				Title:         name,
				Markdown:      mdContent,
				MDContentHash: util.ContentHash(mdContent),
				ModifiedDate:  fileInfo.ModTime(),
				Info:          info,
			}

			posts = append(posts, post)
			postsMap[string(post.ID)] = &post
		}
	}

	slices.SortStableFunc(posts, func(a, b model.Post) int {
		return -a.ModifiedDate.Compare(b.ModifiedDate)
	})

	return posts, postsMap, nil
}

func (r *FSPostRepository) ReadPost(id any) (*model.Post, error) {
	if post, ok := r.postsCache.Get(id.(string)); ok && post.Markdown != nil {
		return post, nil
	}
	return nil, os.ErrNotExist
}

func (r *FSPostRepository) GetAdjacentPosts(id any) (prev *model.Post, next *model.Post) {
	idStr := id.(string)

	// Find the current post in the sorted list
	for i, post := range r.postsCacheSorted {
		if string(post.ID) == idStr {
			// Get previous post (if exists)
			if i > 0 {
				prev = &r.postsCacheSorted[i-1]
			}
			// Get next post (if exists)
			if i < len(r.postsCacheSorted)-1 {
				next = &r.postsCacheSorted[i+1]
			}
			break
		}
	}

	return prev, next
}

func (r *FSPostRepository) ReloadPosts() {
	for {
		posts, postMap, err := r.GetPosts()
		if err != nil {
			repoLogger.Error().Err(err).Msg("Error reloading posts")
		} else {
			for _, post := range r.postsCacheSorted {
				if newPost, ok := postMap[string(post.ID)]; ok {
					if newPost.MDContentHash != post.MDContentHash {
						repoLogger.Info().
							Str("post_id", string(post.ID)).
							Str("title", post.Title).
							Msg("Reloading post")
						go r.notifyPostReload(post.ID)
					}
				}
			}

			r.postsCacheSorted = posts
			r.postsCache.SetTo(postMap)
		}
		time.Sleep(r.reloadTimeout)
	}
}

func (r *FSPostRepository) SetReloadTimeout(timeout time.Duration) {
	r.reloadTimeout = timeout
}

func (r *FSPostRepository) NewPost() *model.Post {
	return &model.Post{}
}

func (r *FSPostRepository) SetPostContent(post *model.Post) error {
	return nil
}

func (r *FSPostRepository) SavePost(post *model.Post) error {
	return nil
}
