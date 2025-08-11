// Package cache provides thread-safe generic caching functionality and markdown rendering cache.
package cache

import "sync"

type Cache[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]V
}

func NewCache[K comparable, V any]() *Cache[K, V] {
	return &Cache[K, V]{
		items: make(map[K]V),
	}
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.items[key]
	return val, ok
}

func (c *Cache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = value
}

func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[K]V)
}

func (c *Cache[K, V]) SetTo(items map[K]V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = items
}

// RenderedContent represents cached rendered markdown with HTML and extra data.
type RenderedContent struct {
	HTML  []byte
	Extra interface{}
}

var renderedMarkdownCache = NewCache[string, *RenderedContent]()

func GetRenderedMarkdown(contentHash, syntaxTheme string) (*RenderedContent, bool) {
	key := contentHash + ":" + syntaxTheme
	return renderedMarkdownCache.Get(key)
}

func SetRenderedMarkdown(contentHash, syntaxTheme string, html []byte, extra interface{}) {
	key := contentHash + ":" + syntaxTheme
	renderedMarkdownCache.Set(key, &RenderedContent{
		HTML:  html,
		Extra: extra,
	})
}

func ClearRenderedMarkdownCache() {
	renderedMarkdownCache.Clear()
}
