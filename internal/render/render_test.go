package render

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/debemdeboas/the-archive/internal/cache"
)

// Test helpers
func setupTest() {
	cache.ClearRenderedMarkdownCache()
}

func assertCacheEntry(t *testing.T, contentHash, syntaxTheme string, expectedHTML []byte, expectedExtra interface{}) {
	t.Helper()
	cached, found := cache.GetRenderedMarkdown(contentHash, syntaxTheme)
	if !found {
		t.Errorf("Expected content to be cached for hash:%s theme:%s", contentHash, syntaxTheme)
		return
	}
	if !bytes.Equal(cached.HTML, expectedHTML) {
		t.Errorf("Cached HTML mismatch. Expected %q, got %q", string(expectedHTML), string(cached.HTML))
	}
	if cached.Extra != expectedExtra {
		t.Errorf("Cached extra data mismatch. Expected %v, got %v", expectedExtra, cached.Extra)
	}
}

func TestRenderMarkdownCached(t *testing.T) {
	tests := []struct {
		name        string
		markdown    []byte
		contentHash string
		syntaxTheme string
		expectCache bool
		expectHTML  bool
	}{
		{
			name:        "basic markdown",
			markdown:    []byte("# Test Header\n\nSome content with `code`"),
			contentHash: "hash-1",
			syntaxTheme: "github",
			expectCache: true,
			expectHTML:  true,
		},
		{
			name:        "empty content",
			markdown:    []byte(""),
			contentHash: "hash-empty",
			syntaxTheme: "github",
			expectCache: true,
			expectHTML:  false,
		},
		{
			name:        "code block with syntax highlighting",
			markdown:    []byte("```go\nfunc main() {\n    fmt.Println(\"Hello\")\n}\n```"),
			contentHash: "hash-code",
			syntaxTheme: "monokai",
			expectCache: true,
			expectHTML:  true,
		},
		{
			name:        "math content",
			markdown:    []byte("Math formula: $E = mc^2$"),
			contentHash: "hash-math",
			syntaxTheme: "github",
			expectCache: true,
			expectHTML:  true,
		},
		{
			name:        "special characters",
			markdown:    []byte("Content with üñíçødé & <script>alert('xss')</script>"),
			contentHash: "hash-special",
			syntaxTheme: "github",
			expectCache: true,
			expectHTML:  true,
		},
	}

	setupTest()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First call - cache miss
			html1, extra1 := RenderMarkdownCached(tt.markdown, tt.contentHash, tt.syntaxTheme)

			if tt.expectHTML && len(html1) == 0 {
				t.Error("Expected rendered HTML, got empty")
			}

			if tt.expectCache {
				assertCacheEntry(t, tt.contentHash, tt.syntaxTheme, html1, extra1)
			}

			// Second call - cache hit
			html2, extra2 := RenderMarkdownCached(tt.markdown, tt.contentHash, tt.syntaxTheme)

			if !bytes.Equal(html1, html2) {
				t.Error("Cache hit should return identical HTML")
			}
			if extra1 != extra2 {
				t.Error("Cache hit should return identical extra data")
			}
		})
	}
}

func TestCacheKeyUniqueness(t *testing.T) {
	setupTest()

	tests := []struct {
		name        string
		contentHash string
		syntaxTheme string
		markdown    []byte
	}{
		{"combo1", "hash-1", "github", []byte("# Test")},
		{"combo2", "hash-1", "monokai", []byte("# Test")},      // Same hash, different theme
		{"combo3", "hash-2", "github", []byte("# Different")},  // Different hash, same theme
		{"combo4", "hash-2", "monokai", []byte("# Different")}, // Both different
	}

	// Render all combinations
	for _, tt := range tests {
		RenderMarkdownCached(tt.markdown, tt.contentHash, tt.syntaxTheme)
	}

	// Verify all are cached separately
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cached, found := cache.GetRenderedMarkdown(tt.contentHash, tt.syntaxTheme)
			if !found {
				t.Error("Expected cache entry to exist")
			}
			if cached == nil {
				t.Error("Expected non-nil cache entry")
			}
		})
	}

	// Verify different combinations have different cache entries
	cached1, _ := cache.GetRenderedMarkdown("hash-1", "github")
	cached2, _ := cache.GetRenderedMarkdown("hash-1", "monokai")
	cached3, _ := cache.GetRenderedMarkdown("hash-2", "github")
	cached4, _ := cache.GetRenderedMarkdown("hash-2", "monokai")

	if cached1 == cached2 || cached1 == cached3 || cached1 == cached4 ||
		cached2 == cached3 || cached2 == cached4 || cached3 == cached4 {
		t.Error("All cache entries should be separate objects")
	}
}

func TestCacheConcurrency(t *testing.T) {
	setupTest()

	const numGoroutines = 100
	const numIterations = 10

	markdown := []byte("# Concurrent Test\n\nContent with `code`")
	contentHash := "concurrent-hash"
	syntaxTheme := "github"

	var wg sync.WaitGroup
	results := make(chan []byte, numGoroutines*numIterations)

	// Start multiple goroutines rendering the same content
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				html, _ := RenderMarkdownCached(markdown, contentHash, syntaxTheme)
				results <- html
			}
		}()
	}

	wg.Wait()
	close(results)

	// Collect all results
	var allResults [][]byte
	for result := range results {
		allResults = append(allResults, result)
	}

	// Verify all results are identical (cache working correctly)
	if len(allResults) != numGoroutines*numIterations {
		t.Fatalf("Expected %d results, got %d", numGoroutines*numIterations, len(allResults))
	}

	firstResult := allResults[0]
	for i, result := range allResults {
		if !bytes.Equal(result, firstResult) {
			t.Errorf("Result %d differs from first result", i)
		}
	}

	// Verify content is cached (don't check extra data in concurrency test)
	cached, found := cache.GetRenderedMarkdown(contentHash, syntaxTheme)
	if !found {
		t.Error("Expected content to be cached")
	}
	if !bytes.Equal(cached.HTML, firstResult) {
		t.Error("Cached HTML should match first result")
	}
}

func TestCacheInvalidation(t *testing.T) {
	setupTest()

	markdown1 := []byte("# Original Content")
	markdown2 := []byte("# Modified Content")
	contentHash1 := "hash-original"
	contentHash2 := "hash-modified"
	syntaxTheme := "github"

	// Cache first content
	html1, extra1 := RenderMarkdownCached(markdown1, contentHash1, syntaxTheme)
	assertCacheEntry(t, contentHash1, syntaxTheme, html1, extra1)

	// Cache second content (simulating content change with new hash)
	html2, extra2 := RenderMarkdownCached(markdown2, contentHash2, syntaxTheme)
	assertCacheEntry(t, contentHash2, syntaxTheme, html2, extra2)

	// Both should be cached with different keys
	if bytes.Equal(html1, html2) {
		t.Error("Different content should produce different HTML")
	}

	// Original content should still be cached
	assertCacheEntry(t, contentHash1, syntaxTheme, html1, extra1)
}

func TestEdgeCases(t *testing.T) {
	setupTest()

	tests := []struct {
		name        string
		markdown    []byte
		contentHash string
		syntaxTheme string
		description string
	}{
		{
			name:        "extremely long content",
			markdown:    []byte(strings.Repeat("# Header\n\nContent paragraph.\n\n", 1000)),
			contentHash: "hash-long",
			syntaxTheme: "github",
			description: "Should handle large content efficiently",
		},
		{
			name:        "mixed line endings",
			markdown:    []byte("# Title\r\n\r\nContent\r\nMore content\n\nEnd"),
			contentHash: "hash-mixed-endings",
			syntaxTheme: "github",
			description: "Should handle mixed line endings",
		},
		{
			name:        "nested code blocks",
			markdown:    []byte("```markdown\n# Header\n```go\nfunc main() {}\n```\n```"),
			contentHash: "hash-nested",
			syntaxTheme: "monokai",
			description: "Should handle nested code blocks",
		},
		{
			name:        "unicode content",
			markdown:    []byte("# 测试 🚀\n\nContent with emoji 😀 and unicode ñáéíóú"),
			contentHash: "hash-unicode",
			syntaxTheme: "github",
			description: "Should handle unicode content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			html, extra := RenderMarkdownCached(tt.markdown, tt.contentHash, tt.syntaxTheme)

			if len(html) == 0 {
				t.Errorf("Expected HTML output for case: %s", tt.description)
			}

			// Verify caching works
			assertCacheEntry(t, tt.contentHash, tt.syntaxTheme, html, extra)

			// Verify cache hit returns same content
			html2, extra2 := RenderMarkdownCached(tt.markdown, tt.contentHash, tt.syntaxTheme)
			if !bytes.Equal(html, html2) {
				t.Error("Cache hit should return identical HTML")
			}
			if extra != extra2 {
				t.Error("Cache hit should return identical extra data")
			}
		})
	}
}

func BenchmarkRenderMarkdownCached(b *testing.B) {
	cache.ClearRenderedMarkdownCache()

	markdown := []byte(`# Performance Test
	
This is a test document with some **bold text** and *italic text*.

Here's some code:

` + "```go" + `
func main() {
    fmt.Println("Hello, World!")
    for i := 0; i < 10; i++ {
        fmt.Printf("Count: %d\n", i)
    }
}
` + "```" + `

And some math: $E = mc^2$

More content here to make it substantial.
`)

	contentHash := "perf-test-hash"
	syntaxTheme := "github"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		RenderMarkdownCached(markdown, contentHash, syntaxTheme)
	}
}

func BenchmarkRenderMarkdownUncached(b *testing.B) {
	markdown := []byte(`# Performance Test
	
This is a test document with some **bold text** and *italic text*.

Here's some code:

` + "```go" + `
func main() {
    fmt.Println("Hello, World!")
    for i := 0; i < 10; i++ {
        fmt.Printf("Count: %d\n", i)
    }
}
` + "```" + `

And some math: $E = mc^2$

More content here to make it substantial.
`)

	syntaxTheme := "github"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		RenderMarkdown(markdown, syntaxTheme)
	}
}

func BenchmarkCacheHitVsMiss(b *testing.B) {
	setupTest()

	markdown := []byte("# Simple test content\n\nWith some text.")
	contentHash := "bench-hash"
	syntaxTheme := "github"

	// Pre-populate cache
	RenderMarkdownCached(markdown, contentHash, syntaxTheme)

	b.Run("CacheHit", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			RenderMarkdownCached(markdown, contentHash, syntaxTheme)
		}
	})

	b.Run("CacheMiss", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Use different hash each time to force cache miss
			RenderMarkdownCached(markdown, fmt.Sprintf("hash-%d", i), syntaxTheme)
		}
	})
}
