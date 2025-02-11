package repository

import (
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/debemdeboas/the-archive/internal/cache"
	"github.com/debemdeboas/the-archive/internal/model"
	"github.com/debemdeboas/the-archive/internal/util"
)

type FSPostRepository struct { // implements PostRepository
	postsPath string

	postsCache       *cache.Cache[string, *model.Post]
	postsCacheSorted []model.Post

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
		log.Fatal("Error initializing posts:", err)
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

			mdContent, err := r.ReadPost(name)
			if err != nil {
				return nil, nil, err
			}

			fileInfo, err := entry.Info()
			if err != nil {
				return nil, nil, err
			}

			post := model.Post{
				Title:         name,
				Path:          name,
				MDContentHash: util.ContentHash(mdContent),
				ModifiedDate:  fileInfo.ModTime(),
			}

			posts = append(posts, post)
			postsMap[name] = &post
		}
	}

	slices.SortStableFunc(posts, func(a, b model.Post) int {
		return -a.ModifiedDate.Compare(b.ModifiedDate)
	})

	return posts, postsMap, nil
}

func (r *FSPostRepository) ReadPost(path any) ([]byte, error) {
	return os.ReadFile(filepath.Join(r.postsPath, path.(string)+".md"))
}

func (r *FSPostRepository) ReloadPosts() {
	for {
		posts, postMap, err := r.GetPosts()
		if err != nil {
			log.Println("Error reloading posts:", err)
		} else {
			for _, post := range r.postsCacheSorted {
				if newPost, ok := postMap[post.Path]; ok {
					if newPost.MDContentHash != post.MDContentHash {
						log.Printf("Reloading post: %s", post.Path)
						go r.notifyPostReload(model.PostID(post.Path))
					}
				}
			}

			r.postsCacheSorted = posts
			r.postsCache.SetTo(postMap)
		}
		time.Sleep(5 * time.Second)
	}
}
