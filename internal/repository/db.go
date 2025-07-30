package repository

import (
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
			// Check if any posts have changed by comparing content hashes
			hasChanges := false

			// Create a map of current cached posts for quick lookup
			cachedPosts := make(map[string]*model.Post)
			for i := range r.postsCacheSorted {
				cachedPosts[string(r.postsCacheSorted[i].Id)] = &r.postsCacheSorted[i]
			}

			// Check for new or modified posts
			for _, newPost := range posts {
				if cachedPost, exists := cachedPosts[string(newPost.Id)]; exists {
					// Compare content hashes to detect changes
					if newPost.MDContentHash != cachedPost.MDContentHash {
						hasChanges = true
						repoLogger.Info().
							Str("post_id", string(newPost.Id)).
							Str("title", newPost.Title).
							Msg("Post content changed, reloading")
						if r.reloadNotifier != nil {
							go r.reloadNotifier(newPost.Id)
						}
					}
				} else {
					// New post detected
					hasChanges = true
					repoLogger.Info().
						Str("post_id", string(newPost.Id)).
						Str("title", newPost.Title).
						Msg("New post detected")
				}
			}

			// Check for deleted posts
			if len(posts) != len(r.postsCacheSorted) {
				hasChanges = true
				repoLogger.Info().Msg("Number of posts changed")
			}

			if hasChanges {
				repoLogger.Info().Msg("Posts have changed, updating cache")
				r.postsCacheSorted = posts
				r.postsCache.SetTo(postMap)
			}
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
