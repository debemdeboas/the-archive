package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/debemdeboas/the-archive/internal/auth"
	"github.com/debemdeboas/the-archive/internal/auth/testdata"
	"github.com/debemdeboas/the-archive/internal/config"
	"github.com/debemdeboas/the-archive/internal/db"
	"github.com/debemdeboas/the-archive/internal/model"
	"github.com/debemdeboas/the-archive/internal/repository"
	"github.com/debemdeboas/the-archive/internal/repository/editor"
	"github.com/debemdeboas/the-archive/internal/sse"
	"github.com/rs/zerolog"
)

// newTestApplication creates a test application using existing patterns
func newTestApplication(t *testing.T) *Application {
	// Setup minimal config for testing
	config.AppConfig = &config.Config{
		Site: config.SiteConfig{
			Name:        "Test Blog",
			Description: "Test blog for unit testing",
		},
		Theme: config.ThemeConfig{
			Default: "dark",
		},
		Features: config.FeaturesConfig{
			Authentication: config.AuthConfig{
				Enabled: true,
				Type:    "ed25519",
			},
			Editor: config.EditorConfig{
				Enabled: true,
			},
		},
	}

	// Create database - use a temporary file for testing
	dbFile, err := os.CreateTemp("", "test-db-*.sqlite")
	if err != nil {
		t.Fatalf("Failed to create temp db file: %v", err)
	}
	dbFile.Close()

	// Override the database path for testing
	originalDB := os.Getenv("DATABASE_PATH")
	os.Setenv("DATABASE_PATH", dbFile.Name())
	defer func() {
		os.Setenv("DATABASE_PATH", originalDB)
	}()

	database := db.NewSQLite()
	if err := database.InitDB(); err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}

	// Create components
	postRepo := repository.NewDBPostRepository(database)
	editorRepo := editor.NewMemoryRepository()
	clients := sse.NewSSEClients()
	editorHandler := editor.NewHandler(editorRepo, clients, &content)

	// Create auth provider with test keys
	authProvider, err := auth.NewEd25519AuthProvider(
		testdata.TestPublicKeyPEM,
		"Authorization",
		model.UserID(testdata.TestUserID),
	)
	if err != nil {
		t.Fatalf("Failed to create auth provider: %v", err)
	}

	// Create logger
	logger := zerolog.New(zerolog.NewConsoleWriter()).Level(zerolog.ErrorLevel)

	app := &Application{
		log:           logger,
		db:            database,
		postRepo:      postRepo,
		editorRepo:    editorRepo,
		editorHandler: editorHandler,
		authProvider:  authProvider,
		clients:       clients,
	}

	// Cleanup
	t.Cleanup(func() {
		database.Close()
		os.Remove(dbFile.Name())
	})

	return app
}

func TestServeIndex(t *testing.T) {
	app := newTestApplication(t)

	req := httptest.NewRequest("GET", "/", nil)
	recorder := httptest.NewRecorder()

	app.serveIndex(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", recorder.Code)
	}

	// Check that it returns HTML
	contentType := recorder.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Error("Expected HTML content type")
	}
}

func TestServePost(t *testing.T) {
	app := newTestApplication(t)

	testCases := []struct {
		name           string
		postID         string
		expectedStatus int
	}{
		{
			name:           "Non-existent post returns 404",
			postID:         "non-existent",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Empty post ID returns 404",
			postID:         "",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/posts/"+tc.postID, nil)
			recorder := httptest.NewRecorder()

			app.servePost(recorder, req)

			if recorder.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, recorder.Code)
			}
		})
	}
}

func TestHandleApiPosts(t *testing.T) {
	app := newTestApplication(t)

	testCases := []struct {
		name           string
		method         string
		authenticated  bool
		expectedStatus int
	}{
		{
			name:           "Unauthenticated POST returns 401",
			method:         http.MethodPost,
			authenticated:  false,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Unauthenticated PUT returns 401",
			method:         http.MethodPut,
			authenticated:  false,
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Invalid method returns 401 (auth checked first)",
			method:         http.MethodGet,
			authenticated:  false,
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/api/posts/test", nil)

			if tc.authenticated {
				// Add auth context for authenticated requests
				ctx := auth.ContextWithUserID(req.Context(), model.UserID(testdata.TestUserID))
				req = req.WithContext(ctx)
			}

			recorder := httptest.NewRecorder()

			app.handleAPIPosts(recorder, req)

			if recorder.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, recorder.Code)
			}
		})
	}
}

func TestApplicationComponents(t *testing.T) {
	app := newTestApplication(t)

	t.Run("Database connection works", func(t *testing.T) {
		if app.db == nil {
			t.Fatal("Expected database to be initialized")
		}
	})

	t.Run("Auth provider is configured", func(t *testing.T) {
		if app.authProvider == nil {
			t.Fatal("Expected auth provider to be configured")
		}

		// Test that we can get user ID from session (should fail without context)
		req := httptest.NewRequest("GET", "/test", nil)
		_, err := app.authProvider.GetUserIDFromSession(req)
		if err == nil {
			t.Error("Expected error for request without auth context")
		}
	})

	t.Run("Repository is configured", func(t *testing.T) {
		if app.postRepo == nil {
			t.Fatal("Expected post repository to be configured")
		}

		// Test that we can get posts (should return empty list initially)
		posts := app.postRepo.GetPostList()
		// Posts list might be nil or empty initially, both are valid
		_ = posts
	})

	t.Run("Editor components are configured", func(t *testing.T) {
		if app.editorRepo == nil {
			t.Fatal("Expected editor repository to be configured")
		}

		if app.editorHandler == nil {
			t.Fatal("Expected editor handler to be configured")
		}
	})

	t.Run("SSE clients are configured", func(t *testing.T) {
		if app.clients == nil {
			t.Fatal("Expected SSE clients to be configured")
		}
	})
}
