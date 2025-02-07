package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServeIndex(t *testing.T) {
	// Set up a dummy cachedPostsSorted for testing.
	cachedPostsSorted = []Post{
		{Title: "Post1", Path: "post1", Content: "<p>Content</p>"},
	}
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	serveIndex(rec, req)

	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 OK, got %d", res.StatusCode)
	}

	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "Post1") {
		t.Errorf("Expected body to contain post title, got %s", body)
	}
}

func TestServePostNotFound(t *testing.T) {
	// Try a post that doesn't exist.
	req := httptest.NewRequest("GET", PostsUrlPath+"nonexistent", nil)
	rec := httptest.NewRecorder()

	servePost(rec, req)

	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found, got %d", res.StatusCode)
	}
}

func TestWebhookReloadHandlerUnauthorized(t *testing.T) {
	// No secret provided should be unauthorized.
	req := httptest.NewRequest("GET", "/webhook/reload", nil)
	rec := httptest.NewRecorder()

	webhookReloadHandler(rec, req)

	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", res.StatusCode)
	}
}

func TestWebhookReloadHandlerRateLimit(t *testing.T) {
	// Provide correct header but send two requests quickly.
	req := httptest.NewRequest("GET", "/webhook/reload", nil)
	req.Header.Set("X-Webhook-Secret", webhookSecret)
	rec := httptest.NewRecorder()

	// First request should work.
	webhookReloadHandler(rec, req)
	res := rec.Result()
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, res.StatusCode)
	}

	// Second request, immediately after, should be rate limited.
	rec2 := httptest.NewRecorder()
	webhookReloadHandler(rec2, req)
	res2 := rec2.Result()
	defer res2.Body.Close()
	if res2.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Expected status %d for rate limited request, got %d", http.StatusTooManyRequests, res2.StatusCode)
	}
}
