package model

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/debemdeboas/the-archive/internal/config"
	"github.com/mmarkdown/mmark/v2/mast"
	"github.com/mmarkdown/mmark/v2/mast/reference"
)

func TestUserID(t *testing.T) {
	t.Run("UserID type operations", func(t *testing.T) {
		var uid UserID = "test-user-123"

		if string(uid) != "test-user-123" {
			t.Errorf("Expected string conversion 'test-user-123', got %s", string(uid))
		}

		// Test comparison
		var uid2 UserID = "test-user-123"
		var uid3 UserID = "different-user"

		if uid != uid2 {
			t.Error("Expected equal UserIDs to be equal")
		}

		if uid == uid3 {
			t.Error("Expected different UserIDs to be different")
		}

		// Test empty UserID
		var emptyUID UserID
		if string(emptyUID) != "" {
			t.Errorf("Expected empty UserID to be empty string, got %s", string(emptyUID))
		}
	})
}

func TestPostID(t *testing.T) {
	t.Run("PostID type operations", func(t *testing.T) {
		var pid PostID = "test-post-456"

		if string(pid) != "test-post-456" {
			t.Errorf("Expected string conversion 'test-post-456', got %s", string(pid))
		}

		// Test comparison
		var pid2 PostID = "test-post-456"
		var pid3 PostID = "different-post"

		if pid != pid2 {
			t.Error("Expected equal PostIDs to be equal")
		}

		if pid == pid3 {
			t.Error("Expected different PostIDs to be different")
		}
	})
}

func TestPost(t *testing.T) {
	t.Run("Post struct creation", func(t *testing.T) {
		now := time.Now()
		post := &Post{
			ID:            "test-post",
			Title:         "Test Post Title",
			Content:       template.HTML("<h1>Test Content</h1>"),
			Path:          "/posts/test-post",
			MDContentHash: "hash123",
			Markdown:      []byte("# Test Content"),
			CreatedDate:   now,
			ModifiedDate:  now.Add(time.Hour),
			Owner:         "test-user",
		}

		if post.ID != "test-post" {
			t.Errorf("Expected ID 'test-post', got %s", post.ID)
		}
		if post.Title != "Test Post Title" {
			t.Errorf("Expected Title 'Test Post Title', got %s", post.Title)
		}
		if string(post.Content) != "<h1>Test Content</h1>" {
			t.Errorf("Expected Content '<h1>Test Content</h1>', got %s", string(post.Content))
		}
		if post.Path != "/posts/test-post" {
			t.Errorf("Expected Path '/posts/test-post', got %s", post.Path)
		}
		if post.MDContentHash != "hash123" {
			t.Errorf("Expected MDContentHash 'hash123', got %s", post.MDContentHash)
		}
		if string(post.Markdown) != "# Test Content" {
			t.Errorf("Expected Markdown '# Test Content', got %s", string(post.Markdown))
		}
		if post.Owner != "test-user" {
			t.Errorf("Expected Owner 'test-user', got %s", post.Owner)
		}
	})

	t.Run("Post with nil Info", func(t *testing.T) {
		post := &Post{
			ID:    "test-post",
			Title: "Fallback Title",
			Info:  nil,
		}

		if post.Info != nil {
			t.Error("Expected Info to be nil")
		}
	})

	t.Run("Post with empty values", func(t *testing.T) {
		post := &Post{}

		if post.ID != "" {
			t.Errorf("Expected empty ID, got %s", post.ID)
		}
		if post.Title != "" {
			t.Errorf("Expected empty Title, got %s", post.Title)
		}
		if post.Owner != "" {
			t.Errorf("Expected empty Owner, got %s", post.Owner)
		}
	})
}

func TestPostGetTitle(t *testing.T) {
	t.Run("GetTitle with no Info returns Title field", func(t *testing.T) {
		post := &Post{
			Title: "Direct Title",
			Info:  nil,
		}

		result := post.GetTitle()
		if result != "Direct Title" {
			t.Errorf("Expected 'Direct Title', got %s", result)
		}
	})

	t.Run("GetTitle with empty Info returns Title field", func(t *testing.T) {
		post := &Post{
			Title: "Direct Title",
			Info:  &mast.TitleData{},
		}

		result := post.GetTitle()
		if result != "Direct Title" {
			t.Errorf("Expected 'Direct Title', got %s", result)
		}
	})

	t.Run("GetTitle with Info.Title returns Info.Title", func(t *testing.T) {
		post := &Post{
			Title: "Direct Title",
			Info: &mast.TitleData{
				Title: "Info Title",
			},
		}

		result := post.GetTitle()
		if result != "Info Title" {
			t.Errorf("Expected 'Info Title', got %s", result)
		}
	})

	t.Run("GetTitle with series info prepends series", func(t *testing.T) {
		post := &Post{
			Title: "Direct Title",
			Info: &mast.TitleData{
				Title: "Episode Title",
				SeriesInfo: reference.SeriesInfo{
					Name:  "MySerial",
					Value: "5",
				},
			},
		}

		result := post.GetTitle()
		expected := "[MySerial-5] Episode Title"
		if result != expected {
			t.Errorf("Expected %q, got %q", expected, result)
		}
	})

	t.Run("GetTitle with partial series info", func(t *testing.T) {
		// Test with only series name
		post1 := &Post{
			Title: "Direct Title",
			Info: &mast.TitleData{
				Title: "Episode Title",
				SeriesInfo: reference.SeriesInfo{
					Name:  "MySerial",
					Value: "",
				},
			},
		}

		result1 := post1.GetTitle()
		if result1 != "Episode Title" {
			t.Errorf("Expected 'Episode Title' (no series), got %q", result1)
		}

		// Test with only series value
		post2 := &Post{
			Title: "Direct Title",
			Info: &mast.TitleData{
				Title: "Episode Title",
				SeriesInfo: reference.SeriesInfo{
					Name:  "",
					Value: "5",
				},
			},
		}

		result2 := post2.GetTitle()
		if result2 != "Episode Title" {
			t.Errorf("Expected 'Episode Title' (no series), got %q", result2)
		}
	})

	t.Run("GetTitle with empty Info.Title returns Title field", func(t *testing.T) {
		post := &Post{
			Title: "Direct Title",
			Info: &mast.TitleData{
				Title: "",
				SeriesInfo: reference.SeriesInfo{
					Name:  "MySerial",
					Value: "5",
				},
			},
		}

		result := post.GetTitle()
		if result != "Direct Title" {
			t.Errorf("Expected 'Direct Title', got %q", result)
		}
	})

	t.Run("GetTitle with complex series info", func(t *testing.T) {
		post := &Post{
			Title: "Direct Title",
			Info: &mast.TitleData{
				Title: "Advanced Go Patterns",
				SeriesInfo: reference.SeriesInfo{
					Name:  "Go-Tutorial",
					Value: "Part-10",
				},
			},
		}

		result := post.GetTitle()
		expected := "[Go-Tutorial-Part-10] Advanced Go Patterns"
		if result != expected {
			t.Errorf("Expected %q, got %q", expected, result)
		}
	})
}

func TestPageData(t *testing.T) {
	// Set up config for tests
	originalConfig := config.AppConfig
	defer func() { config.AppConfig = originalConfig }()

	config.AppConfig = &config.Config{
		Site: config.SiteConfig{
			Name:        "Test Site",
			Tagline:     "Test Tagline",
			Description: "Test Description",
		},
		Meta: config.MetaConfig{
			Keywords: []string{"test", "blog"},
			Author:   "Test Author",
		},
		Theme: config.ThemeConfig{
			AllowSwitching: true,
		},
		Features: config.FeaturesConfig{
			Editor: config.EditorConfig{
				Enabled:     true,
				LivePreview: true,
			},
		},
	}

	t.Run("PageData struct creation", func(t *testing.T) {
		pd := &PageData{
			SiteName:            "Test Site Name",
			SiteTagline:         "Test Tagline",
			SiteDescription:     "Test Description",
			SiteKeywords:        []string{"keyword1", "keyword2"},
			SiteAuthor:          "Test Author",
			PageURL:             "/test/path",
			Theme:               "dark",
			AllowThemeSwitching: true,
			EditorEnabled:       true,
			LivePreviewEnabled:  true,
			SyntaxTheme:         "monokai",
			SyntaxThemes:        []string{"github", "monokai"},
		}

		if pd.SiteName != "Test Site Name" {
			t.Errorf("Expected SiteName 'Test Site Name', got %s", pd.SiteName)
		}
		if pd.PageURL != "/test/path" {
			t.Errorf("Expected PageURL '/test/path', got %s", pd.PageURL)
		}
		if pd.Theme != "dark" {
			t.Errorf("Expected Theme 'dark', got %s", pd.Theme)
		}
		if !pd.AllowThemeSwitching {
			t.Error("Expected AllowThemeSwitching to be true")
		}
		if !pd.EditorEnabled {
			t.Error("Expected EditorEnabled to be true")
		}
		if len(pd.SiteKeywords) != 2 {
			t.Errorf("Expected 2 keywords, got %d", len(pd.SiteKeywords))
		}
	})

	t.Run("PageData with nil pointer fields", func(t *testing.T) {
		pd := &PageData{
			ShowToolbar:  nil,
			IsEditorPage: nil,
		}

		if pd.ShowToolbar != nil {
			t.Error("Expected ShowToolbar to be nil")
		}
		if pd.IsEditorPage != nil {
			t.Error("Expected IsEditorPage to be nil")
		}
	})

	t.Run("PageData with boolean pointers", func(t *testing.T) {
		showToolbar := true
		isEditorPage := false

		pd := &PageData{
			ShowToolbar:  &showToolbar,
			IsEditorPage: &isEditorPage,
		}

		if pd.ShowToolbar == nil || *pd.ShowToolbar != true {
			t.Error("Expected ShowToolbar to be true")
		}
		if pd.IsEditorPage == nil || *pd.IsEditorPage != false {
			t.Error("Expected IsEditorPage to be false")
		}
	})
}

func TestNewPageData(t *testing.T) {
	// Set up config for tests
	originalConfig := config.AppConfig
	defer func() { config.AppConfig = originalConfig }()

	config.AppConfig = &config.Config{
		Site: config.SiteConfig{
			Name:        "Test Site",
			Tagline:     "Test Tagline",
			Description: "Test Description",
		},
		Meta: config.MetaConfig{
			Keywords: []string{"test", "blog"},
			Author:   "Test Author",
		},
		Theme: config.ThemeConfig{
			AllowSwitching: true,
		},
		Features: config.FeaturesConfig{
			Editor: config.EditorConfig{
				Enabled:     true,
				LivePreview: true,
			},
		},
	}

	t.Run("NewPageData creates PageData from request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test/path", nil)

		pd := NewPageData(req)

		if pd == nil {
			t.Fatal("Expected non-nil PageData")
		}

		if pd.SiteName != "Test Site" {
			t.Errorf("Expected SiteName 'Test Site', got %s", pd.SiteName)
		}
		if pd.SiteTagline != "Test Tagline" {
			t.Errorf("Expected SiteTagline 'Test Tagline', got %s", pd.SiteTagline)
		}
		if pd.SiteDescription != "Test Description" {
			t.Errorf("Expected SiteDescription 'Test Description', got %s", pd.SiteDescription)
		}
		if pd.SiteAuthor != "Test Author" {
			t.Errorf("Expected SiteAuthor 'Test Author', got %s", pd.SiteAuthor)
		}
		if pd.PageURL != "/test/path" {
			t.Errorf("Expected PageURL '/test/path', got %s", pd.PageURL)
		}
		if !pd.AllowThemeSwitching {
			t.Error("Expected AllowThemeSwitching to be true")
		}
		if !pd.EditorEnabled {
			t.Error("Expected EditorEnabled to be true")
		}
		if !pd.LivePreviewEnabled {
			t.Error("Expected LivePreviewEnabled to be true")
		}

		// Check that arrays are properly copied
		expectedKeywords := []string{"test", "blog"}
		if len(pd.SiteKeywords) != len(expectedKeywords) {
			t.Errorf("Expected %d keywords, got %d", len(expectedKeywords), len(pd.SiteKeywords))
		}
		for i, keyword := range expectedKeywords {
			if i < len(pd.SiteKeywords) && pd.SiteKeywords[i] != keyword {
				t.Errorf("Expected keyword[%d] %s, got %s", i, keyword, pd.SiteKeywords[i])
			}
		}
	})

	t.Run("NewPageData with theme cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "theme", Value: "light"})

		pd := NewPageData(req)

		// Note: The actual theme determination depends on the theme package
		// We're mainly testing that the function doesn't panic and creates a valid PageData
		if pd == nil {
			t.Fatal("Expected non-nil PageData")
		}
	})

	t.Run("NewPageData with syntax theme cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "syntax-theme", Value: "monokai"})

		pd := NewPageData(req)

		if pd == nil {
			t.Fatal("Expected non-nil PageData")
		}
	})
}

func TestPageDataIsPost(t *testing.T) {
	// Set up config for URL path
	originalConfig := config.AppConfig
	defer func() { config.AppConfig = originalConfig }()

	config.AppConfig = &config.Config{}

	t.Run("IsPost with nil ShowToolbar checks URL", func(t *testing.T) {
		pd := &PageData{
			PageURL:     "/posts/test-post",
			ShowToolbar: nil,
		}

		result := pd.IsPost()
		if !result {
			t.Error("Expected IsPost to be true for posts URL")
		}
	})

	t.Run("IsPost with non-posts URL returns false", func(t *testing.T) {
		pd := &PageData{
			PageURL:     "/about",
			ShowToolbar: nil,
		}

		result := pd.IsPost()
		if result {
			t.Error("Expected IsPost to be false for non-posts URL")
		}
	})

	t.Run("IsPost with ShowToolbar set to true", func(t *testing.T) {
		showToolbar := true
		pd := &PageData{
			PageURL:     "/about",
			ShowToolbar: &showToolbar,
		}

		result := pd.IsPost()
		if !result {
			t.Error("Expected IsPost to be true when ShowToolbar is true")
		}
	})

	t.Run("IsPost with ShowToolbar set to false", func(t *testing.T) {
		showToolbar := false
		pd := &PageData{
			PageURL:     "/posts/test-post",
			ShowToolbar: &showToolbar,
		}

		result := pd.IsPost()
		if result {
			t.Error("Expected IsPost to be false when ShowToolbar is false")
		}
	})
}

func TestPageDataIsEditor(t *testing.T) {
	t.Run("IsEditor with nil IsEditorPage checks URL", func(t *testing.T) {
		pd := &PageData{
			PageURL:      "/new/post/edit",
			IsEditorPage: nil,
		}

		result := pd.IsEditor()
		if !result {
			t.Error("Expected IsEditor to be true for editor URL")
		}
	})

	t.Run("IsEditor with editor URL variation", func(t *testing.T) {
		pd := &PageData{
			PageURL:      "/new/post/edit/something",
			IsEditorPage: nil,
		}

		result := pd.IsEditor()
		if !result {
			t.Error("Expected IsEditor to be true for editor URL with path")
		}
	})

	t.Run("IsEditor with non-editor URL returns false", func(t *testing.T) {
		pd := &PageData{
			PageURL:      "/posts/test-post",
			IsEditorPage: nil,
		}

		result := pd.IsEditor()
		if result {
			t.Error("Expected IsEditor to be false for non-editor URL")
		}
	})

	t.Run("IsEditor with IsEditorPage set to true", func(t *testing.T) {
		isEditorPage := true
		pd := &PageData{
			PageURL:      "/posts/test-post",
			IsEditorPage: &isEditorPage,
		}

		result := pd.IsEditor()
		if !result {
			t.Error("Expected IsEditor to be true when IsEditorPage is true")
		}
	})

	t.Run("IsEditor with IsEditorPage set to false", func(t *testing.T) {
		isEditorPage := false
		pd := &PageData{
			PageURL:      "/new/post/edit",
			IsEditorPage: &isEditorPage,
		}

		result := pd.IsEditor()
		if result {
			t.Error("Expected IsEditor to be false when IsEditorPage is false")
		}
	})
}

func TestPageDataEdgeCases(t *testing.T) {
	t.Run("PageData with empty URL paths", func(t *testing.T) {
		pd := &PageData{
			PageURL: "",
		}

		if pd.IsPost() {
			t.Error("Expected IsPost to be false for empty URL")
		}
		if pd.IsEditor() {
			t.Error("Expected IsEditor to be false for empty URL")
		}
	})

	t.Run("PageData with root URL", func(t *testing.T) {
		pd := &PageData{
			PageURL: "/",
		}

		if pd.IsPost() {
			t.Error("Expected IsPost to be false for root URL")
		}
		if pd.IsEditor() {
			t.Error("Expected IsEditor to be false for root URL")
		}
	})

	t.Run("PageData with case sensitivity", func(t *testing.T) {
		pd := &PageData{
			PageURL: "/POSTS/test-post",
		}

		// URLs are case-sensitive, so this should be false
		if pd.IsPost() {
			t.Error("Expected IsPost to be false for uppercase POSTS")
		}
	})

	t.Run("PageData with partial matches", func(t *testing.T) {
		pd1 := &PageData{PageURL: "/post/test"} // Doesn't start with /posts/
		if pd1.IsPost() {
			t.Error("Expected IsPost to be false for /post/ (no 's')")
		}

		pd2 := &PageData{PageURL: "/new/post/editor"} // Doesn't start with /new/post/edit
		if pd2.IsEditor() {
			t.Error("Expected IsEditor to be false for /new/post/editor")
		}

		pd3 := &PageData{PageURL: "/new/post/edit"} // Should be true
		if !pd3.IsEditor() {
			t.Error("Expected IsEditor to be true for /new/post/edit")
		}
	})
}

func TestModelPackageIntegration(t *testing.T) {
	t.Run("Post and PageData together", func(t *testing.T) {
		// Create a post
		post := &Post{
			ID:    "integration-test",
			Title: "Integration Test Post",
			Info: &mast.TitleData{
				Title: "Advanced Integration Testing",
				SeriesInfo: reference.SeriesInfo{
					Name:  "Testing",
					Value: "1",
				},
			},
		}

		// Create page data
		req := httptest.NewRequest("GET", "/posts/integration-test", nil)

		// Set up minimal config
		originalConfig := config.AppConfig
		defer func() { config.AppConfig = originalConfig }()

		config.AppConfig = &config.Config{
			Site: config.SiteConfig{Name: "Test Site"},
			Meta: config.MetaConfig{Keywords: []string{"test"}},
			Features: config.FeaturesConfig{
				Editor: config.EditorConfig{Enabled: true},
			},
		}

		pd := NewPageData(req)

		// Test integration
		if !pd.IsPost() {
			t.Error("Expected page to be identified as post")
		}

		title := post.GetTitle()
		if !strings.Contains(title, "Testing-1") {
			t.Errorf("Expected title to contain series info, got %s", title)
		}

		// UserID integration
		var userID UserID = "test-user"
		post.Owner = userID

		if post.Owner != userID {
			t.Error("Expected post owner to match UserID")
		}
	})
}
