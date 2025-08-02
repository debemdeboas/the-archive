package repository

import (
	"database/sql"
	"testing"

	"github.com/debemdeboas/the-archive/internal/model"
	_ "github.com/mattn/go-sqlite3"
)

// Mock database for testing
type testDB struct {
	*sql.DB
}

func (t *testDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return t.DB.Query(query, args...)
}

func (t *testDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return t.DB.Exec(query, args...)
}

func (t *testDB) Get() *sql.DB {
	return t.DB
}

func (t *testDB) Close() error {
	return t.DB.Close()
}

func (t *testDB) InitDB() error {
	_, err := t.DB.Exec(`
		CREATE TABLE IF NOT EXISTS posts (
			id TEXT PRIMARY KEY,
			title TEXT,
			content BLOB,
			md_content_hash TEXT,
			created_at DATETIME,
			modified_at DATETIME,
			user_id TEXT
		)
	`)
	return err
}

func setupTestDB() (*testDB, error) {
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, err
	}

	testDB := &testDB{DB: sqlDB}
	err = testDB.InitDB()
	if err != nil {
		return nil, err
	}

	return testDB, nil
}

func TestReloadPostsHashComparison(t *testing.T) {
	// Setup test database
	testDB, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test database: %v", err)
	}
	defer testDB.Close()

	// Create repository
	repo := NewDBPostRepository(testDB)

	// Create initial post
	post1 := repo.NewPost()
	post1.Title = "Test Post 1"
	post1.Markdown = []byte("# Hello World")
	post1.Owner = model.UserID("test-user")

	err = repo.SavePost(post1)
	if err != nil {
		t.Fatalf("Failed to save initial post: %v", err)
	}

	// Initialize repository cache
	posts, postMap, err := repo.GetPosts()
	if err != nil {
		t.Fatalf("Failed to get posts: %v", err)
	}
	repo.postsCacheSorted = posts
	repo.postsCache.SetTo(postMap)

	if len(posts) != 1 {
		t.Fatalf("Expected 1 post, got %d", len(posts))
	}

	// Track reload notifications
	reloadCalled := false
	var reloadedPostID model.PostID
	repo.SetReloadNotifier(func(postID model.PostID) {
		reloadCalled = true
		reloadedPostID = postID
	})

	// Test 1: No changes should not trigger reload
	t.Run("NoChanges", func(t *testing.T) {
		reloadCalled = false

		// Simulate one iteration of ReloadPosts logic
		newPosts, _, err := repo.GetPosts()
		if err != nil {
			t.Fatalf("Failed to get posts: %v", err)
		}

		// Check if any posts have changed by comparing content hashes
		hasChanges := false
		cachedPosts := make(map[string]*model.Post)
		for i := range repo.postsCacheSorted {
			cachedPosts[string(repo.postsCacheSorted[i].ID)] = &repo.postsCacheSorted[i]
		}

		for _, newPost := range newPosts {
			if cachedPost, exists := cachedPosts[string(newPost.ID)]; exists {
				if newPost.MDContentHash != cachedPost.MDContentHash {
					hasChanges = true
					break
				}
			} else {
				hasChanges = true
				break
			}
		}

		if hasChanges {
			t.Error("Expected no changes, but changes were detected")
		}
		if reloadCalled {
			t.Error("Reload notification should not have been called")
		}
	})

	// Test 2: Content change should trigger reload
	t.Run("ContentChange", func(t *testing.T) {
		reloadCalled = false

		// Modify the post content
		post1.Markdown = []byte("# Hello World Modified!")
		err = repo.SetPostContent(post1)
		if err != nil {
			t.Fatalf("Failed to update post: %v", err)
		}

		// Simulate one iteration of ReloadPosts logic
		newPosts, newPostMap, err := repo.GetPosts()
		if err != nil {
			t.Fatalf("Failed to get posts: %v", err)
		}

		// Check if any posts have changed by comparing content hashes
		hasChanges := false
		cachedPosts := make(map[string]*model.Post)
		for i := range repo.postsCacheSorted {
			cachedPosts[string(repo.postsCacheSorted[i].ID)] = &repo.postsCacheSorted[i]
		}

		var changedPostID model.PostID
		for _, newPost := range newPosts {
			if cachedPost, exists := cachedPosts[string(newPost.ID)]; exists {
				if newPost.MDContentHash != cachedPost.MDContentHash {
					hasChanges = true
					changedPostID = newPost.ID
					// Simulate the reload notification
					if repo.reloadNotifier != nil {
						repo.reloadNotifier(newPost.ID)
					}
					break
				}
			}
		}

		if !hasChanges {
			t.Error("Expected changes to be detected, but none were found")
		}
		if !reloadCalled {
			t.Error("Reload notification should have been called")
		}
		if reloadedPostID != changedPostID {
			t.Errorf("Expected reload notification for post %s, got %s", changedPostID, reloadedPostID)
		}

		// Update the cache to reflect changes
		repo.postsCacheSorted = newPosts
		repo.postsCache.SetTo(newPostMap)
	})

	// Test 3: New post should trigger reload
	t.Run("NewPost", func(t *testing.T) {
		reloadCalled = false

		// Create a new post
		post2 := repo.NewPost()
		post2.Title = "Test Post 2"
		post2.Markdown = []byte("# Another Post")
		post2.Owner = model.UserID("test-user")

		err = repo.SavePost(post2)
		if err != nil {
			t.Fatalf("Failed to save new post: %v", err)
		}

		// Simulate one iteration of ReloadPosts logic
		newPosts, _, err := repo.GetPosts()
		if err != nil {
			t.Fatalf("Failed to get posts: %v", err)
		}

		// Check for new posts
		cachedPosts := make(map[string]*model.Post)
		for i := range repo.postsCacheSorted {
			cachedPosts[string(repo.postsCacheSorted[i].ID)] = &repo.postsCacheSorted[i]
		}

		hasNewPosts := false
		for _, newPost := range newPosts {
			if _, exists := cachedPosts[string(newPost.ID)]; !exists {
				hasNewPosts = true
				break
			}
		}

		if !hasNewPosts {
			t.Error("Expected new post to be detected, but none were found")
		}
		if len(newPosts) != 2 {
			t.Errorf("Expected 2 posts, got %d", len(newPosts))
		}
	})
}

func TestHashComparison(t *testing.T) {
	// Test that different content produces different hashes
	testDB, err := setupTestDB()
	if err != nil {
		t.Fatalf("Failed to setup test database: %v", err)
	}
	defer testDB.Close()

	repo := NewDBPostRepository(testDB)

	post1 := repo.NewPost()
	post1.Title = "Test"
	post1.Markdown = []byte("Content 1")
	post1.Owner = model.UserID("test")

	post2 := repo.NewPost()
	post2.Title = "Test"
	post2.Markdown = []byte("Content 2")
	post2.Owner = model.UserID("test")

	err = repo.SavePost(post1)
	if err != nil {
		t.Fatalf("Failed to save post1: %v", err)
	}

	err = repo.SavePost(post2)
	if err != nil {
		t.Fatalf("Failed to save post2: %v", err)
	}

	posts, _, err := repo.GetPosts()
	if err != nil {
		t.Fatalf("Failed to get posts: %v", err)
	}

	if len(posts) != 2 {
		t.Fatalf("Expected 2 posts, got %d", len(posts))
	}

	// Different content should have different hashes
	if posts[0].MDContentHash == posts[1].MDContentHash {
		t.Error("Different content should produce different hashes")
	}

	// Same content should have same hash
	post3 := repo.NewPost()
	post3.Title = "Test"
	post3.Markdown = []byte("Content 1") // Same as post1
	post3.Owner = model.UserID("test")

	err = repo.SavePost(post3)
	if err != nil {
		t.Fatalf("Failed to save post3: %v", err)
	}

	posts, _, err = repo.GetPosts()
	if err != nil {
		t.Fatalf("Failed to get posts: %v", err)
	}

	// Find post1 and post3 and compare hashes
	var post1Hash, post3Hash string
	for _, post := range posts {
		if post.ID == post1.ID {
			post1Hash = post.MDContentHash
		}
		if post.ID == post3.ID {
			post3Hash = post.MDContentHash
		}
	}

	if post1Hash != post3Hash {
		t.Error("Same content should produce same hashes")
	}
}
