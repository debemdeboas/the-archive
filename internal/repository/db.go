package repository

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"slices"
	"time"

	"github.com/debemdeboas/the-archive/internal/cache"
	"github.com/debemdeboas/the-archive/internal/db"
	"github.com/debemdeboas/the-archive/internal/model"
	"github.com/debemdeboas/the-archive/internal/util"
	"github.com/debemdeboas/the-archive/internal/util/compression"
	"github.com/google/uuid"
)

type DbPostRepository struct { // implements PostRepository
	postsCache       *cache.Cache[string, *model.Post]
	postsCacheSorted []model.Post

	reloadNotifier func(model.PostId)

	db         db.Db
	compressor compression.Compressor
}

func NewDbPostRepository(db db.Db) *DbPostRepository {
	return &DbPostRepository{
		postsCache: cache.NewCache[string, *model.Post](),

		db: db,

		compressor: compression.ZstdCompressor{},
	}
}

func (r *DbPostRepository) Init() {
	posts, postMap, err := r.GetPosts()
	if err != nil {
		repoLogger.Fatal().Err(err).Msg("Error initializing posts")
	}

	r.postsCacheSorted = posts
	r.postsCache.SetTo(postMap)

	go r.ReloadPosts()
}

func (r *DbPostRepository) GetPosts() ([]model.Post, map[string]*model.Post, error) {
	rows, err := r.db.Query(`SELECT id, title, content, md_content_hash, created_at, modified_at, user_id FROM posts`)
	if err != nil {
		return nil, nil, fmt.Errorf("error querying posts: %w", err)
	}
	defer rows.Close()

	posts := make([]model.Post, 0)
	postMap := make(map[string]*model.Post)

	for rows.Next() {
		var post model.Post
		var compressed []byte

		err := rows.Scan(&post.Id, &post.Title, &compressed, &post.MDContentHash, &post.CreatedDate, &post.ModifiedDate, &post.Owner)
		if err != nil {
			return nil, nil, fmt.Errorf("error scanning post: %w", err)
		}

		// Decompress the content
		content, err := r.compressor.Decompress(compressed)
		if err != nil {
			return nil, nil, fmt.Errorf("error decompressing content: %w", err)
		}
		post.Markdown = content

		posts = append(posts, post)
		postMap[string(post.Id)] = &post
	}

	// Sort the posts by creation date
	slices.SortStableFunc(posts, func(a, b model.Post) int {
		return -a.ModifiedDate.Compare(b.ModifiedDate)
	})

	return posts, postMap, nil
}

func (r *DbPostRepository) GetPostList() []model.Post {
	return r.postsCacheSorted
}

func (r *DbPostRepository) ReadPost(id any) (*model.Post, error) {
	post, ok := r.postsCache.Get(id.(string))
	if !ok {
		return nil, fmt.Errorf("post not found: %s", id)
	}
	return post, nil
}

func (r *DbPostRepository) ReloadPosts() {
	for {
		posts, postMap, err := r.GetPosts()
		if err != nil {
			repoLogger.Error().Err(err).Msg("Error reloading posts")
		} else {
			var curHashBuf bytes.Buffer
			gob.NewEncoder(&curHashBuf).Encode(posts)

			var oldHashBuf bytes.Buffer
			gob.NewEncoder(&oldHashBuf).Encode(r.postsCacheSorted)

			if curHashBuf.String() != oldHashBuf.String() {
				repoLogger.Info().Msg("Posts have changed, reloading")
				// Try to find which posts have changed
				for _, post := range r.postsCacheSorted {
					if newPost, ok := postMap[string(post.Id)]; ok {
						var newHashBuf bytes.Buffer
						gob.NewEncoder(&newHashBuf).Encode(newPost)

						var oldHashBuf bytes.Buffer
						gob.NewEncoder(&oldHashBuf).Encode(post)

						if newHashBuf.String() != oldHashBuf.String() {
							repoLogger.Info().
								Str("post_id", string(post.Id)).
								Str("title", post.Title).
								Msg("Reloading post")
							if r.reloadNotifier != nil {
								go r.reloadNotifier(post.Id)
							}
						}
					}
				}
			}

			r.postsCacheSorted = posts
			r.postsCache.SetTo(postMap)
		}

		time.Sleep(10 * time.Second)
	}
}

func (r *DbPostRepository) SetReloadNotifier(notifier func(model.PostId)) {
	r.reloadNotifier = notifier
}

func (r *DbPostRepository) NewPost() *model.Post {
	now := time.Now().UTC()

	return &model.Post{
		Id: model.PostId(uuid.New().String()),

		CreatedDate:  now,
		ModifiedDate: now,
	}
}

func (r *DbPostRepository) SetPostContent(post *model.Post) error {
	// Compress the content
	compressed, err := r.compressor.Compress([]byte(post.Markdown))
	if err != nil {
		return fmt.Errorf("error compressing content: %w", err)
	}

	// Calculate the content hash for the compressed content
	post.MDContentHash = util.ContentHash(compressed)

	// Save the post
	res, err := r.db.Exec(
		`UPDATE posts SET title = ?, content = ?, md_content_hash = ?, modified_at = ? WHERE id = ?`,
		post.Title, compressed, post.MDContentHash, time.Now(), post.Id,
	)

	if err != nil {
		return fmt.Errorf("error saving post: %w", err)
	}

	fmt.Println(res)

	return nil
}

func (r *DbPostRepository) SavePost(post *model.Post) error {
	// Compress the content
	compressed, err := r.compressor.Compress([]byte(post.Markdown))
	if err != nil {
		return fmt.Errorf("error compressing content: %w", err)
	}

	// Calculate the content hash for the compressed content
	post.MDContentHash = util.ContentHash(compressed)

	// Save the post
	res, err := r.db.Exec(
		`INSERT INTO posts (id, title, content, md_content_hash, created_at, modified_at, user_id) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		post.Id, post.Title, compressed, post.MDContentHash, post.CreatedDate, post.ModifiedDate, post.Owner,
	)

	if err != nil {
		return fmt.Errorf("error saving post: %w", err)
	}

	fmt.Println(res)

	return nil
}
