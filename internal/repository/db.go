package repository

import (
	"database/sql"
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

type DBPostRepository struct { // implements PostRepository
	postsCache       *cache.Cache[string, *model.Post]
	postsCacheSorted []model.Post

	reloadNotifier   func(model.PostID)
	lastModifiedTime *time.Time // Track the latest modification time

	db         db.DB
	compressor compression.Compressor
}

func NewDBPostRepository(db db.DB) *DBPostRepository {
	return &DBPostRepository{
		postsCache: cache.NewCache[string, *model.Post](),

		db: db,

		compressor: compression.ZstdCompressor{},
	}
}

func (r *DBPostRepository) Init() {
	posts, postMap, err := r.GetPosts()
	if err != nil {
		repoLogger.Fatal().Err(err).Msg("Error initializing posts")
	}

	r.postsCacheSorted = posts
	r.postsCache.SetTo(postMap)

	go r.ReloadPosts()
}

func (r *DBPostRepository) GetLatestModifiedTime() (*time.Time, error) {
	var latestTimeStr sql.NullString
	row := r.db.Get().QueryRow(`SELECT MAX(modified_at) FROM posts`)
	err := row.Scan(&latestTimeStr)
	if err != nil {
		return nil, fmt.Errorf("error scanning latest modified time: %w", err)
	}

	if !latestTimeStr.Valid {
		return nil, nil // It was NULL, so no posts or no valid timestamps.
	}

	// The go-sqlite3 driver returns a string for MAX(), so we must parse it.
	// It can be in a format with a space separator.
	timeFormats := []string{
		"2006-01-02 15:04:05.999999999-07:00", // Space separator with timezone
		time.RFC3339Nano,                      // 'T' separator with timezone
		time.RFC3339,                          // 'T' separator, no nanos
	}

	var latestTime time.Time
	var parseErr error
	for _, format := range timeFormats {
		latestTime, parseErr = time.Parse(format, latestTimeStr.String)
		if parseErr == nil {
			return &latestTime, nil
		}
	}

	return nil, fmt.Errorf("error parsing latest modified time '%s' with any known format: %w", latestTimeStr.String, parseErr)
}

func (r *DBPostRepository) GetPosts() ([]model.Post, map[string]*model.Post, error) {
	rows, err := r.db.Query(`SELECT id, title, content, md_content_hash, created_at, modified_at, user_id FROM posts`)
	if err != nil {
		return nil, nil, fmt.Errorf("error querying posts: %w", err)
	}
	defer rows.Close()

	posts := make([]model.Post, 0)
	postMap := make(map[string]*model.Post)
	var latestModTime *time.Time

	for rows.Next() {
		var post model.Post
		var compressed []byte

		err := rows.Scan(&post.ID, &post.Title, &compressed, &post.MDContentHash, &post.CreatedDate, &post.ModifiedDate, &post.Owner)
		if err != nil {
			return nil, nil, fmt.Errorf("error scanning post: %w", err)
		}

		// Track the latest modification time
		if latestModTime == nil || post.ModifiedDate.After(*latestModTime) {
			latestModTime = &post.ModifiedDate
		}

		// Decompress the content
		content, err := r.compressor.Decompress(compressed)
		if err != nil {
			return nil, nil, fmt.Errorf("error decompressing content: %w", err)
		}
		post.Markdown = content

		posts = append(posts, post)
		postMap[string(post.ID)] = &post
	}

	// Update our tracked modification time
	r.lastModifiedTime = latestModTime

	// Sort the posts by creation date
	slices.SortStableFunc(posts, func(a, b model.Post) int {
		return -a.ModifiedDate.Compare(b.ModifiedDate)
	})

	return posts, postMap, nil
}

func (r *DBPostRepository) GetPostList() []model.Post {
	return r.postsCacheSorted
}

func (r *DBPostRepository) ReadPost(id any) (*model.Post, error) {
	post, ok := r.postsCache.Get(id.(string))
	if !ok {
		return nil, fmt.Errorf("post not found: %s", id)
	}
	return post, nil
}

func (r *DBPostRepository) ReloadPosts() {
	sleepFunc := func() {
		time.Sleep(10 * time.Second)
	}

	for {
		// First, do a lightweight check to see if anything has changed
		latestTime, err := r.GetLatestModifiedTime()
		if err != nil {
			repoLogger.Error().Err(err).Msg("Error checking latest modification time")
			sleepFunc()
			continue
		}

		// If we have a cached time and nothing has changed, skip
		if r.lastModifiedTime != nil && latestTime != nil && !latestTime.After(*r.lastModifiedTime) {
			repoLogger.Debug().Msg("No posts modified, skipping reload")
			sleepFunc()
			continue
		}

		repoLogger.Debug().Msg("Posts may have changed, performing full reload")

		// Something changed, do the full reload
		posts, postMap, err := r.GetPosts()
		if err != nil {
			repoLogger.Error().Err(err).Msg("Error reloading posts")
		} else {
			// Check if any posts have changed by comparing content hashes
			hasChanges := false

			// Create a map of current cached posts for quick lookup
			cachedPosts := make(map[string]*model.Post)
			for i := range r.postsCacheSorted {
				cachedPosts[string(r.postsCacheSorted[i].ID)] = &r.postsCacheSorted[i]
			}

			// Check for new or modified posts
			for _, newPost := range posts {
				if cachedPost, exists := cachedPosts[string(newPost.ID)]; exists {
					// Compare content hashes to detect changes
					if newPost.MDContentHash != cachedPost.MDContentHash {
						hasChanges = true
						repoLogger.Info().
							Str("post_id", string(newPost.ID)).
							Str("title", newPost.Title).
							Msg("Post content changed, reloading")
						if r.reloadNotifier != nil {
							go r.reloadNotifier(newPost.ID)
						}
					}
				} else {
					// New post detected
					hasChanges = true
					repoLogger.Info().
						Str("post_id", string(newPost.ID)).
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

		sleepFunc()
	}
}

func (r *DBPostRepository) SetReloadNotifier(notifier func(model.PostID)) {
	r.reloadNotifier = notifier
}

func (r *DBPostRepository) NewPost() *model.Post {
	now := time.Now().UTC()

	return &model.Post{
		ID: model.PostID(uuid.New().String()),

		CreatedDate:  now,
		ModifiedDate: now,
	}
}

func (r *DBPostRepository) SetPostContent(post *model.Post) error {
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
		post.Title, compressed, post.MDContentHash, time.Now().UTC(), post.ID,
	)

	if err != nil {
		return fmt.Errorf("error saving post: %w", err)
	}

	repoLogger.Debug().Interface("result", res).Msg("Post content set")

	return nil
}

func (r *DBPostRepository) SavePost(post *model.Post) error {
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
		post.ID, post.Title, compressed, post.MDContentHash, post.CreatedDate, post.ModifiedDate, post.Owner,
	)

	if err != nil {
		return fmt.Errorf("error saving post: %w", err)
	}

	repoLogger.Debug().Interface("result", res).Msg("Post saved")

	return nil
}
