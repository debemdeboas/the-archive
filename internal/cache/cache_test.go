package cache

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
)

func TestNewCache(t *testing.T) {
	t.Run("String cache", func(t *testing.T) {
		cache := NewCache[string, string]()
		if cache == nil {
			t.Fatal("Expected non-nil cache")
		}
		if cache.items == nil {
			t.Fatal("Expected items map to be initialized")
		}
	})

	t.Run("Integer cache", func(t *testing.T) {
		cache := NewCache[int, string]()
		if cache == nil {
			t.Fatal("Expected non-nil cache")
		}
	})

	t.Run("Complex types cache", func(t *testing.T) {
		type TestStruct struct {
			ID   int
			Name string
		}
		cache := NewCache[string, *TestStruct]()
		if cache == nil {
			t.Fatal("Expected non-nil cache")
		}
	})
}

func TestCache_BasicOperations(t *testing.T) {
	cache := NewCache[string, string]()

	t.Run("Set and Get", func(t *testing.T) {
		key := "test-key"
		value := "test-value"

		// Set value
		cache.Set(key, value)

		// Get value
		got, exists := cache.Get(key)
		if !exists {
			t.Error("Expected key to exist")
		}
		if got != value {
			t.Errorf("Expected %q, got %q", value, got)
		}
	})

	t.Run("Get non-existent key", func(t *testing.T) {
		_, exists := cache.Get("non-existent")
		if exists {
			t.Error("Expected key to not exist")
		}
	})

	t.Run("Overwrite existing key", func(t *testing.T) {
		key := "overwrite-key"
		value1 := "value1"
		value2 := "value2"

		cache.Set(key, value1)
		cache.Set(key, value2)

		got, exists := cache.Get(key)
		if !exists {
			t.Error("Expected key to exist")
		}
		if got != value2 {
			t.Errorf("Expected %q, got %q", value2, got)
		}
	})
}

func TestCache_Delete(t *testing.T) {
	cache := NewCache[string, string]()

	t.Run("Delete existing key", func(t *testing.T) {
		key := "delete-key"
		value := "delete-value"

		cache.Set(key, value)
		cache.Delete(key)

		_, exists := cache.Get(key)
		if exists {
			t.Error("Expected key to be deleted")
		}
	})

	t.Run("Delete non-existent key", func(t *testing.T) {
		// Should not panic
		cache.Delete("non-existent")
	})
}

func TestCache_Clear(t *testing.T) {
	cache := NewCache[string, string]()

	t.Run("Clear populated cache", func(t *testing.T) {
		// Add multiple items
		cache.Set("key1", "value1")
		cache.Set("key2", "value2")
		cache.Set("key3", "value3")

		// Clear cache
		cache.Clear()

		// Verify all items are gone
		_, exists1 := cache.Get("key1")
		_, exists2 := cache.Get("key2")
		_, exists3 := cache.Get("key3")

		if exists1 || exists2 || exists3 {
			t.Error("Expected all keys to be cleared")
		}
	})

	t.Run("Clear empty cache", func(t *testing.T) {
		cache.Clear() // Should not panic
	})
}

func TestCache_SetTo(t *testing.T) {
	cache := NewCache[string, string]()

	t.Run("SetTo with new items", func(t *testing.T) {
		newItems := map[string]string{
			"new1": "value1",
			"new2": "value2",
			"new3": "value3",
		}

		cache.SetTo(newItems)

		for key, expectedValue := range newItems {
			got, exists := cache.Get(key)
			if !exists {
				t.Errorf("Expected key %q to exist", key)
			}
			if got != expectedValue {
				t.Errorf("For key %q, expected %q, got %q", key, expectedValue, got)
			}
		}
	})

	t.Run("SetTo replaces existing items", func(t *testing.T) {
		// Add initial items
		cache.Set("old1", "oldvalue1")
		cache.Set("old2", "oldvalue2")

		// Replace with new items
		newItems := map[string]string{
			"new1": "newvalue1",
			"new2": "newvalue2",
		}
		cache.SetTo(newItems)

		// Old items should be gone
		_, exists1 := cache.Get("old1")
		_, exists2 := cache.Get("old2")
		if exists1 || exists2 {
			t.Error("Expected old items to be replaced")
		}

		// New items should exist
		got1, exists1 := cache.Get("new1")
		got2, exists2 := cache.Get("new2")
		if !exists1 || !exists2 {
			t.Error("Expected new items to exist")
		}
		if got1 != "newvalue1" || got2 != "newvalue2" {
			t.Error("Expected new values to be set correctly")
		}
	})

	t.Run("SetTo with empty map", func(t *testing.T) {
		cache.Set("test", "value")
		cache.SetTo(map[string]string{})

		_, exists := cache.Get("test")
		if exists {
			t.Error("Expected cache to be empty after SetTo with empty map")
		}
	})
}

func TestCache_Concurrency(t *testing.T) {
	cache := NewCache[int, string]()
	const numGoroutines = 100
	const numOperations = 1000

	t.Run("Concurrent reads and writes", func(t *testing.T) {
		var wg sync.WaitGroup

		// Writers
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					key := id*numOperations + j
					value := fmt.Sprintf("value-%d-%d", id, j)
					cache.Set(key, value)
				}
			}(i)
		}

		// Readers
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					key := id*numOperations + j
					cache.Get(key) // Don't check result as it may not exist yet
				}
			}(i)
		}

		wg.Wait()
	})

	t.Run("Concurrent deletes", func(t *testing.T) {
		// Pre-populate cache
		for i := 0; i < 1000; i++ {
			cache.Set(i, fmt.Sprintf("value-%d", i))
		}

		var wg sync.WaitGroup
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					key := id*10 + j
					cache.Delete(key)
				}
			}(i)
		}

		wg.Wait()
	})

	t.Run("Concurrent clear operations", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				cache.Clear()
			}()
		}

		wg.Wait()
	})
}

func TestCache_TypeSafety(t *testing.T) {
	t.Run("String to int cache", func(t *testing.T) {
		cache := NewCache[string, int]()
		cache.Set("number", 42)

		got, exists := cache.Get("number")
		if !exists {
			t.Error("Expected key to exist")
		}
		if got != 42 {
			t.Errorf("Expected 42, got %d", got)
		}
	})

	t.Run("Complex struct cache", func(t *testing.T) {
		type User struct {
			ID   int
			Name string
			Age  int
		}

		cache := NewCache[int, *User]()
		user := &User{ID: 1, Name: "Alice", Age: 30}

		cache.Set(1, user)

		got, exists := cache.Get(1)
		if !exists {
			t.Error("Expected user to exist")
		}
		if got.ID != 1 || got.Name != "Alice" || got.Age != 30 {
			t.Errorf("Expected user data to match, got %+v", got)
		}
	})
}

func TestRenderedMarkdownCache(t *testing.T) {
	// Clear cache before each test
	ClearRenderedMarkdownCache()

	t.Run("Set and get rendered markdown", func(t *testing.T) {
		contentHash := "test-hash"
		syntaxTheme := "github"
		html := []byte("<h1>Test</h1>")
		extra := "test-extra"

		SetRenderedMarkdown(contentHash, syntaxTheme, html, extra)

		cached, found := GetRenderedMarkdown(contentHash, syntaxTheme)
		if !found {
			t.Error("Expected cached content to be found")
		}
		if !bytes.Equal(cached.HTML, html) {
			t.Errorf("Expected HTML %q, got %q", string(html), string(cached.HTML))
		}
		if cached.Extra != extra {
			t.Errorf("Expected extra %v, got %v", extra, cached.Extra)
		}
	})

	t.Run("Different content hash creates separate entries", func(t *testing.T) {
		syntaxTheme := "monokai"
		html1 := []byte("<h1>Content 1</h1>")
		html2 := []byte("<h1>Content 2</h1>")

		SetRenderedMarkdown("hash1", syntaxTheme, html1, "extra1")
		SetRenderedMarkdown("hash2", syntaxTheme, html2, "extra2")

		cached1, found1 := GetRenderedMarkdown("hash1", syntaxTheme)
		cached2, found2 := GetRenderedMarkdown("hash2", syntaxTheme)

		if !found1 || !found2 {
			t.Error("Expected both cached contents to be found")
		}
		if bytes.Equal(cached1.HTML, cached2.HTML) {
			t.Error("Expected different HTML content for different hashes")
		}
	})

	t.Run("Different syntax theme creates separate entries", func(t *testing.T) {
		contentHash := "same-hash"
		html := []byte("<h1>Same Content</h1>")

		SetRenderedMarkdown(contentHash, "github", html, "github-extra")
		SetRenderedMarkdown(contentHash, "monokai", html, "monokai-extra")

		cached1, found1 := GetRenderedMarkdown(contentHash, "github")
		cached2, found2 := GetRenderedMarkdown(contentHash, "monokai")

		if !found1 || !found2 {
			t.Error("Expected both cached contents to be found")
		}
		if cached1.Extra == cached2.Extra {
			t.Error("Expected different extra data for different themes")
		}
	})

	t.Run("Key construction", func(t *testing.T) {
		// Test that the key is constructed as contentHash:syntaxTheme
		contentHash := "test:hash"  // Hash with colon
		syntaxTheme := "theme:name" // Theme with colon
		html := []byte("<p>Test</p>")

		SetRenderedMarkdown(contentHash, syntaxTheme, html, nil)

		cached, found := GetRenderedMarkdown(contentHash, syntaxTheme)
		if !found {
			t.Error("Expected cached content to be found even with colons in values")
		}
		if !bytes.Equal(cached.HTML, html) {
			t.Error("Expected HTML to match")
		}
	})

	t.Run("Clear rendered markdown cache", func(t *testing.T) {
		// Add some entries
		SetRenderedMarkdown("hash1", "theme1", []byte("html1"), "extra1")
		SetRenderedMarkdown("hash2", "theme2", []byte("html2"), "extra2")

		// Clear cache
		ClearRenderedMarkdownCache()

		// Verify entries are gone
		_, found1 := GetRenderedMarkdown("hash1", "theme1")
		_, found2 := GetRenderedMarkdown("hash2", "theme2")

		if found1 || found2 {
			t.Error("Expected all cached content to be cleared")
		}
	})

	t.Run("Get non-existent cached content", func(t *testing.T) {
		_, found := GetRenderedMarkdown("non-existent", "theme")
		if found {
			t.Error("Expected non-existent content to not be found")
		}
	})
}

func TestRenderedMarkdownCache_Concurrency(t *testing.T) {
	ClearRenderedMarkdownCache()

	const numGoroutines = 50
	const numOperations = 100

	t.Run("Concurrent markdown cache operations", func(t *testing.T) {
		var wg sync.WaitGroup

		// Writers
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					contentHash := fmt.Sprintf("hash-%d-%d", id, j)
					syntaxTheme := fmt.Sprintf("theme-%d", id%5) // Rotate through 5 themes
					html := []byte(fmt.Sprintf("<h1>Content %d-%d</h1>", id, j))
					extra := fmt.Sprintf("extra-%d-%d", id, j)

					SetRenderedMarkdown(contentHash, syntaxTheme, html, extra)
				}
			}(i)
		}

		// Readers
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					contentHash := fmt.Sprintf("hash-%d-%d", id, j)
					syntaxTheme := fmt.Sprintf("theme-%d", id%5)
					GetRenderedMarkdown(contentHash, syntaxTheme)
				}
			}(i)
		}

		wg.Wait()
	})

	t.Run("Concurrent clear operations", func(t *testing.T) {
		// Pre-populate cache
		for i := 0; i < 100; i++ {
			SetRenderedMarkdown(fmt.Sprintf("hash-%d", i), "theme", []byte("html"), "extra")
		}

		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ClearRenderedMarkdownCache()
			}()
		}

		wg.Wait()
	})
}

func BenchmarkCache_Set(b *testing.B) {
	cache := NewCache[int, string]()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(i, fmt.Sprintf("value-%d", i))
	}
}

func BenchmarkCache_Get(b *testing.B) {
	cache := NewCache[int, string]()

	// Pre-populate cache
	for i := 0; i < 10000; i++ {
		cache.Set(i, fmt.Sprintf("value-%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(i % 10000)
	}
}

func BenchmarkRenderedMarkdownCache_Set(b *testing.B) {
	html := []byte("<h1>Benchmark Test</h1><p>Some content here</p>")
	extra := "benchmark-extra"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SetRenderedMarkdown(fmt.Sprintf("hash-%d", i), "github", html, extra)
	}
}

func BenchmarkRenderedMarkdownCache_Get(b *testing.B) {
	html := []byte("<h1>Benchmark Test</h1><p>Some content here</p>")

	// Pre-populate cache
	for i := 0; i < 10000; i++ {
		SetRenderedMarkdown(fmt.Sprintf("hash-%d", i), "github", html, "extra")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetRenderedMarkdown(fmt.Sprintf("hash-%d", i%10000), "github")
	}
}

func BenchmarkCache_ConcurrentReadWrite(b *testing.B) {
	cache := NewCache[int, string]()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				cache.Set(i, fmt.Sprintf("value-%d", i))
			} else {
				cache.Get(i)
			}
			i++
		}
	})
}
