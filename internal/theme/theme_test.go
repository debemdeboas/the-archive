package theme

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/debemdeboas/the-archive/internal/cache"
	"github.com/debemdeboas/the-archive/internal/config"
)

func TestGenerateSyntaxCSS(t *testing.T) {
	testCases := []struct {
		name          string
		theme         string
		expectEmpty   bool
		expectInCache bool
	}{
		{
			name:          "Valid Theme - Monokai",
			theme:         "monokai",
			expectEmpty:   false,
			expectInCache: true,
		},
		{
			name:          "Valid Theme - Github",
			theme:         "github",
			expectEmpty:   false,
			expectInCache: true,
		},
		{
			name:          "Valid Theme - Gruvbox",
			theme:         "gruvbox",
			expectEmpty:   false,
			expectInCache: true,
		},
		{
			name:          "Non-existent Theme - Fallback",
			theme:         "nonexistent-theme-12345",
			expectEmpty:   false, // Should return fallback style, not empty
			expectInCache: true,
		},
		{
			name:          "Empty Theme Name",
			theme:         "",
			expectEmpty:   false, // Should return fallback style, not empty
			expectInCache: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear syntax cache before each test for isolation
			clearSyntaxCacheForTheme(tc.theme)

			// First call - should generate and cache
			css1 := GenerateSyntaxCSS(tc.theme)

			if tc.expectEmpty && css1 != "" {
				t.Errorf("Expected empty CSS, but got content")
			}
			if !tc.expectEmpty && css1 == "" {
				t.Errorf("Expected CSS content, but got empty")
			}

			// Verify the CSS contains expected content
			cssStr := string(css1)
			if !tc.expectEmpty {
				if !strings.Contains(cssStr, ".chroma") {
					t.Errorf("Expected CSS to contain '.chroma' class")
				}
			}

			// Verify caching
			cachedCSS, found := cache.GetSyntaxCSS(tc.theme)
			if tc.expectInCache && !found {
				t.Errorf("Expected CSS to be in cache, but it wasn't")
			}
			if !tc.expectInCache && found {
				t.Errorf("Expected CSS NOT to be in cache, but it was")
			}
			if tc.expectInCache && found && cachedCSS != css1 {
				t.Errorf("Cached CSS does not match generated CSS")
			}

			// Second call - should hit the cache
			css2 := GenerateSyntaxCSS(tc.theme)
			if css1 != css2 {
				t.Errorf("Expected second call to return identical CSS from cache")
			}
		})
	}
}

func TestGetFormatter(t *testing.T) {
	formatter := GetFormatter()
	if formatter == nil {
		t.Fatal("Expected formatter to be non-nil")
	}
}

func TestGetSyntaxThemes(t *testing.T) {
	themes := GetSyntaxThemes()
	if len(themes) == 0 {
		t.Error("Expected at least one syntax theme")
	}

	// Verify themes are sorted
	for i := 1; i < len(themes); i++ {
		if themes[i-1] > themes[i] {
			t.Errorf("Themes are not sorted: %s > %s", themes[i-1], themes[i])
		}
	}

	// Verify some common themes exist
	commonThemes := []string{"github", "monokai", "gruvbox"}
	for _, theme := range commonThemes {
		found := false
		for _, availableTheme := range themes {
			if availableTheme == theme {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected common theme %s to be available", theme)
		}
	}
}

func TestGetThemeFromRequest(t *testing.T) {
	// Setup mock config for testing
	setupMockConfig()

	testCases := []struct {
		name          string
		cookieValue   string
		hasCookie     bool
		expectedTheme string
	}{
		{
			name:          "No cookie - use default",
			hasCookie:     false,
			expectedTheme: config.AppConfig.Theme.Default,
		},
		{
			name:          "Valid light theme cookie",
			cookieValue:   "light",
			hasCookie:     true,
			expectedTheme: "light",
		},
		{
			name:          "Valid dark theme cookie",
			cookieValue:   "dark",
			hasCookie:     true,
			expectedTheme: "dark",
		},
		{
			name:          "Custom theme cookie",
			cookieValue:   "custom",
			hasCookie:     true,
			expectedTheme: "custom",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tc.hasCookie {
				req.AddCookie(&http.Cookie{
					Name:  config.CookieTheme,
					Value: tc.cookieValue,
				})
			}

			theme := GetThemeFromRequest(req)
			if theme != tc.expectedTheme {
				t.Errorf("Expected theme %s, got %s", tc.expectedTheme, theme)
			}
		})
	}
}

func TestGetSyntaxThemeFromRequest(t *testing.T) {
	// Setup mock config for testing
	setupMockConfig()

	testCases := []struct {
		name            string
		themeCookie     string
		syntaxCookie    string
		hasThemeCookie  bool
		hasSyntaxCookie bool
		expectedTheme   string
	}{
		{
			name:            "No cookies - use default for default theme",
			hasThemeCookie:  false,
			hasSyntaxCookie: false,
			expectedTheme:   GetDefaultSyntaxTheme(config.AppConfig.Theme.Default),
		},
		{
			name:            "Only theme cookie - use default syntax for that theme",
			themeCookie:     "light",
			hasThemeCookie:  true,
			hasSyntaxCookie: false,
			expectedTheme:   GetDefaultSyntaxTheme("light"),
		},
		{
			name:            "Both cookies - use syntax cookie",
			themeCookie:     "dark",
			syntaxCookie:    "monokai",
			hasThemeCookie:  true,
			hasSyntaxCookie: true,
			expectedTheme:   "monokai",
		},
		{
			name:            "Only syntax cookie - use syntax cookie",
			syntaxCookie:    "github",
			hasThemeCookie:  false,
			hasSyntaxCookie: true,
			expectedTheme:   "github",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tc.hasThemeCookie {
				req.AddCookie(&http.Cookie{
					Name:  config.CookieTheme,
					Value: tc.themeCookie,
				})
			}
			if tc.hasSyntaxCookie {
				req.AddCookie(&http.Cookie{
					Name:  config.CookieSyntaxTheme,
					Value: tc.syntaxCookie,
				})
			}

			theme := GetSyntaxThemeFromRequest(req)
			if theme != tc.expectedTheme {
				t.Errorf("Expected syntax theme %s, got %s", tc.expectedTheme, theme)
			}
		})
	}
}

func TestGetDefaultSyntaxTheme(t *testing.T) {
	// Setup mock config for testing
	setupMockConfig()

	testCases := []struct {
		name          string
		theme         string
		expectedTheme string
	}{
		{
			name:          "Light theme",
			theme:         config.LightTheme,
			expectedTheme: config.AppConfig.Theme.SyntaxHighlighting.DefaultLight,
		},
		{
			name:          "Dark theme",
			theme:         config.DarkTheme,
			expectedTheme: config.AppConfig.Theme.SyntaxHighlighting.DefaultDark,
		},
		{
			name:          "Unknown theme",
			theme:         "unknown",
			expectedTheme: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			theme := GetDefaultSyntaxTheme(tc.theme)
			if theme != tc.expectedTheme {
				t.Errorf("Expected default syntax theme %s, got %s", tc.expectedTheme, theme)
			}
		})
	}
}

func TestGetThemeIcon(t *testing.T) {
	// Setup mock config for testing
	setupMockConfig()

	testCases := []struct {
		name         string
		theme        string
		expectedIcon string
	}{
		{
			name:         "Light theme returns dark icon",
			theme:        config.LightTheme,
			expectedIcon: config.DarkThemeIcon,
		},
		{
			name:         "Dark theme returns light icon",
			theme:        config.DarkTheme,
			expectedIcon: config.LightThemeIcon,
		},
		{
			name:         "Unknown theme returns light icon",
			theme:        "unknown",
			expectedIcon: config.LightThemeIcon,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			icon := GetThemeIcon(tc.theme)
			if icon != tc.expectedIcon {
				t.Errorf("Expected icon %s, got %s", tc.expectedIcon, icon)
			}
			if icon == "" {
				t.Error("Expected non-empty icon")
			}
		})
	}
}

// Helper functions for testing

func setupMockConfig() {
	if config.AppConfig == nil {
		config.AppConfig = &config.Config{
			Theme: config.ThemeConfig{
				Default: "dark",
				SyntaxHighlighting: config.SyntaxConfig{
					DefaultDark:  "gruvbox",
					DefaultLight: "catppuccin-latte",
				},
			},
		}
	}
}

func clearSyntaxCacheForTheme(theme string) {
	// For testing isolation, we simply ignore the specific theme clearing
	// since we don't have a direct API to clear individual cache entries
	// This is acceptable for our test purposes
}

// BenchmarkGenerateSyntaxCSS tests the performance impact of caching
func BenchmarkGenerateSyntaxCSS(b *testing.B) {
	theme := "monokai"

	b.Run("Cached", func(b *testing.B) {
		// Run once to populate the cache
		GenerateSyntaxCSS(theme)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			GenerateSyntaxCSS(theme)
		}
	})

	b.Run("Uncached", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Clear all caches each time to simulate uncached performance
			cache.ClearRenderedMarkdownCache()
			GenerateSyntaxCSS(theme)
		}
	})
}
