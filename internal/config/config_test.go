package config

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

func TestSetLogger(t *testing.T) {
	logger := zerolog.New(os.Stdout).Level(zerolog.InfoLevel)
	SetLogger(logger)

	// Verify logger is set (we can't easily compare loggers directly)
	// This test mainly ensures the function doesn't panic
}

func TestApplyDefaults(t *testing.T) {
	t.Run("Config struct defaults", func(t *testing.T) {
		config := &Config{}
		applyDefaults(config)

		// Test Site defaults
		if config.Site.Name != "The Archive" {
			t.Errorf("Expected site name 'The Archive', got %q", config.Site.Name)
		}
		if config.Site.Description != "A personal blog and knowledge archive" {
			t.Errorf("Expected default description, got %q", config.Site.Description)
		}
		if config.Site.Tagline != "Welcome to The Archive" {
			t.Errorf("Expected default tagline, got %q", config.Site.Tagline)
		}

		// Test Server defaults
		if config.Server.Host != "0.0.0.0" {
			t.Errorf("Expected host '0.0.0.0', got %q", config.Server.Host)
		}
		if config.Server.Port != "12600" {
			t.Errorf("Expected port '12600', got %q", config.Server.Port)
		}

		// Test Theme defaults
		if config.Theme.Default != "dark" {
			t.Errorf("Expected theme 'dark', got %q", config.Theme.Default)
		}
		if !config.Theme.AllowSwitching {
			t.Error("Expected theme switching to be enabled by default")
		}
		if config.Theme.SyntaxHighlighting.DefaultDark != "gruvbox" {
			t.Errorf("Expected dark syntax theme 'gruvbox', got %q", config.Theme.SyntaxHighlighting.DefaultDark)
		}
		if config.Theme.SyntaxHighlighting.DefaultLight != "catppuccin-latte" {
			t.Errorf("Expected light syntax theme 'catppuccin-latte', got %q", config.Theme.SyntaxHighlighting.DefaultLight)
		}

		// Test Content defaults
		if config.Content.PostsPerPage != 50 {
			t.Errorf("Expected posts per page 50, got %d", config.Content.PostsPerPage)
		}

		// Test Features defaults
		if !config.Features.Authentication.Enabled {
			t.Error("Expected authentication to be enabled by default")
		}
		if config.Features.Authentication.Type != "ed25519" {
			t.Errorf("Expected auth type 'ed25519', got %q", config.Features.Authentication.Type)
		}
		if !config.Features.Editor.Enabled {
			t.Error("Expected editor to be enabled by default")
		}
		if !config.Features.Editor.LivePreview {
			t.Error("Expected live preview to be enabled by default")
		}
		if config.Features.Search.Enabled {
			t.Error("Expected search to be disabled by default")
		}
		if config.Features.Comments.Enabled {
			t.Error("Expected comments to be disabled by default")
		}

		// Test Meta defaults
		if config.Meta.Author != "" {
			t.Errorf("Expected empty author, got %q", config.Meta.Author)
		}
		expectedKeywords := []string{"blog", "archive", "personal"}
		if !reflect.DeepEqual(config.Meta.Keywords, expectedKeywords) {
			t.Errorf("Expected keywords %v, got %v", expectedKeywords, config.Meta.Keywords)
		}
		if config.Meta.Favicon != "/static/favicon.ico" {
			t.Errorf("Expected favicon '/static/favicon.ico', got %q", config.Meta.Favicon)
		}

		// Test Social defaults (all should be empty)
		if config.Social.GitHub != "" {
			t.Errorf("Expected empty GitHub, got %q", config.Social.GitHub)
		}
		if config.Social.Twitter != "" {
			t.Errorf("Expected empty Twitter, got %q", config.Social.Twitter)
		}
		if config.Social.LinkedIn != "" {
			t.Errorf("Expected empty LinkedIn, got %q", config.Social.LinkedIn)
		}
		if config.Social.Email != "" {
			t.Errorf("Expected empty Email, got %q", config.Social.Email)
		}

		// Test Logging defaults
		if config.Logging.Level != "info" {
			t.Errorf("Expected logging level 'info', got %q", config.Logging.Level)
		}
	})

	t.Run("Custom struct with various field types", func(t *testing.T) {
		type TestStruct struct {
			StringField  string   `default:"test-string"`
			BoolField    bool     `default:"true"`
			IntField     int      `default:"42"`
			Float64Field float64  `default:"3.14"`
			SliceField   []string `default:"a,b,c"`
			NoDefault    string   // No default tag
		}

		test := &TestStruct{}
		applyDefaults(test)

		if test.StringField != "test-string" {
			t.Errorf("Expected string field 'test-string', got %q", test.StringField)
		}
		if !test.BoolField {
			t.Error("Expected bool field to be true")
		}
		if test.IntField != 42 {
			t.Errorf("Expected int field 42, got %d", test.IntField)
		}
		if test.Float64Field != 3.14 {
			t.Errorf("Expected float64 field 3.14, got %f", test.Float64Field)
		}
		expectedSlice := []string{"a", "b", "c"}
		if !reflect.DeepEqual(test.SliceField, expectedSlice) {
			t.Errorf("Expected slice %v, got %v", expectedSlice, test.SliceField)
		}
		if test.NoDefault != "" {
			t.Errorf("Expected no default field to be empty, got %q", test.NoDefault)
		}
	})

	t.Run("Invalid default values", func(t *testing.T) {
		type InvalidStruct struct {
			BadBool  bool    `default:"not-a-bool"`
			BadInt   int     `default:"not-an-int"`
			BadFloat float64 `default:"not-a-float"`
		}

		test := &InvalidStruct{}
		applyDefaults(test) // Should not panic

		// Invalid defaults should leave fields with zero values
		if test.BadBool {
			t.Error("Expected invalid bool default to remain false")
		}
		if test.BadInt != 0 {
			t.Errorf("Expected invalid int default to remain 0, got %d", test.BadInt)
		}
		if test.BadFloat != 0.0 {
			t.Errorf("Expected invalid float default to remain 0.0, got %f", test.BadFloat)
		}
	})

	t.Run("Nested struct defaults", func(t *testing.T) {
		type Inner struct {
			InnerField string `default:"inner-value"`
		}
		type Outer struct {
			OuterField  string `default:"outer-value"`
			InnerStruct Inner
		}

		test := &Outer{}
		applyDefaults(test)

		if test.OuterField != "outer-value" {
			t.Errorf("Expected outer field 'outer-value', got %q", test.OuterField)
		}
		if test.InnerStruct.InnerField != "inner-value" {
			t.Errorf("Expected inner field 'inner-value', got %q", test.InnerStruct.InnerField)
		}
	})

	t.Run("Non-struct input", func(t *testing.T) {
		// Should not panic with non-struct inputs
		stringVar := "test"
		applyDefaults(&stringVar)
		applyDefaults(stringVar)
		applyDefaults(42)
		applyDefaults(nil)
	})
}

func TestLoadConfig(t *testing.T) {
	// Set up logger for testing
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel) // Use error level to reduce test output
	SetLogger(logger)

	t.Run("Load non-existent config file", func(t *testing.T) {
		originalAppConfig := AppConfig
		defer func() { AppConfig = originalAppConfig }()

		err := LoadConfig("non-existent-config.yaml")
		if err != nil {
			t.Errorf("Expected no error for non-existent config file, got %v", err)
		}

		if AppConfig == nil {
			t.Fatal("Expected AppConfig to be set with defaults")
		}

		// Verify defaults were applied
		if AppConfig.Site.Name != "The Archive" {
			t.Errorf("Expected default site name, got %q", AppConfig.Site.Name)
		}
	})

	t.Run("Load valid config file", func(t *testing.T) {
		originalAppConfig := AppConfig
		defer func() { AppConfig = originalAppConfig }()

		// Create temporary config file
		configContent := `
site:
  name: "Test Blog"
  description: "Test Description"
server:
  host: "127.0.0.1"
  port: "8080"
theme:
  default: "light"
  allow_switching: false
content:
  posts_per_page: 25
`
		tempFile, err := os.CreateTemp("", "test-config-*.yaml")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())

		if _, err := tempFile.WriteString(configContent); err != nil {
			t.Fatalf("Failed to write config content: %v", err)
		}
		tempFile.Close()

		err = LoadConfig(tempFile.Name())
		if err != nil {
			t.Fatalf("Expected no error loading valid config, got %v", err)
		}

		if AppConfig == nil {
			t.Fatal("Expected AppConfig to be set")
		}

		// Verify loaded values
		if AppConfig.Site.Name != "Test Blog" {
			t.Errorf("Expected site name 'Test Blog', got %q", AppConfig.Site.Name)
		}
		if AppConfig.Site.Description != "Test Description" {
			t.Errorf("Expected description 'Test Description', got %q", AppConfig.Site.Description)
		}
		if AppConfig.Server.Host != "127.0.0.1" {
			t.Errorf("Expected host '127.0.0.1', got %q", AppConfig.Server.Host)
		}
		if AppConfig.Server.Port != "8080" {
			t.Errorf("Expected port '8080', got %q", AppConfig.Server.Port)
		}
		if AppConfig.Theme.Default != "light" {
			t.Errorf("Expected theme 'light', got %q", AppConfig.Theme.Default)
		}
		if AppConfig.Theme.AllowSwitching {
			t.Error("Expected theme switching to be disabled")
		}
		if AppConfig.Content.PostsPerPage != 25 {
			t.Errorf("Expected posts per page 25, got %d", AppConfig.Content.PostsPerPage)
		}

		// Verify defaults were still applied for unspecified fields
		if AppConfig.Site.Tagline != "Welcome to The Archive" {
			t.Errorf("Expected default tagline, got %q", AppConfig.Site.Tagline)
		}
	})

	t.Run("Load invalid YAML file", func(t *testing.T) {
		originalAppConfig := AppConfig
		defer func() { AppConfig = originalAppConfig }()

		// Create temporary invalid config file
		invalidContent := `
site:
  name: "Test Blog"
  invalid yaml syntax [
`
		tempFile, err := os.CreateTemp("", "test-config-invalid-*.yaml")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())

		if _, err := tempFile.WriteString(invalidContent); err != nil {
			t.Fatalf("Failed to write config content: %v", err)
		}
		tempFile.Close()

		err = LoadConfig(tempFile.Name())
		if err == nil {
			t.Error("Expected error loading invalid config file")
		}
		if !strings.Contains(err.Error(), "failed to parse config file") {
			t.Errorf("Expected parse error, got %v", err)
		}
	})

	t.Run("Partial config with defaults", func(t *testing.T) {
		originalAppConfig := AppConfig
		defer func() { AppConfig = originalAppConfig }()

		// Create config with only some fields
		configContent := `
site:
  name: "Partial Config"
features:
  authentication:
    enabled: false
`
		tempFile, err := os.CreateTemp("", "test-config-partial-*.yaml")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tempFile.Name())

		if _, err := tempFile.WriteString(configContent); err != nil {
			t.Fatalf("Failed to write config content: %v", err)
		}
		tempFile.Close()

		err = LoadConfig(tempFile.Name())
		if err != nil {
			t.Fatalf("Expected no error loading partial config, got %v", err)
		}

		// Verify specified values
		if AppConfig.Site.Name != "Partial Config" {
			t.Errorf("Expected site name 'Partial Config', got %q", AppConfig.Site.Name)
		}
		if AppConfig.Features.Authentication.Enabled {
			t.Error("Expected authentication to be disabled")
		}

		// Verify defaults were applied for unspecified fields
		if AppConfig.Site.Description != "A personal blog and knowledge archive" {
			t.Errorf("Expected default description, got %q", AppConfig.Site.Description)
		}
		if AppConfig.Server.Port != "12600" {
			t.Errorf("Expected default port, got %q", AppConfig.Server.Port)
		}
	})
}

func TestPublicApplyDefaults(t *testing.T) {
	// Test the public ApplyDefaults function
	type TestStruct struct {
		Field string `default:"test-value"`
	}

	test := &TestStruct{}
	ApplyDefaults(test)

	if test.Field != "test-value" {
		t.Errorf("Expected field 'test-value', got %q", test.Field)
	}
}

func TestConstants(t *testing.T) {
	t.Run("Markdown constants", func(t *testing.T) {
		if MarkdownRenderer != "mmark" {
			t.Errorf("Expected MarkdownRenderer 'mmark', got %q", MarkdownRenderer)
		}

		// Test regex compilation doesn't panic
		if RegexCallout == nil {
			t.Error("Expected RegexCallout to be compiled")
		}

		// Test regex functionality
		testString := "// <<1>>"
		matches := RegexCallout.FindStringSubmatch(testString)
		if len(matches) != 2 || matches[1] != "1" {
			t.Errorf("Expected callout regex to match '1', got %v", matches)
		}
	})

	t.Run("Path constants", func(t *testing.T) {
		if StaticLocalDir != "static" {
			t.Errorf("Expected StaticLocalDir 'static', got %q", StaticLocalDir)
		}
		if StaticURLPath != "/static/" {
			t.Errorf("Expected StaticURLPath '/static/', got %q", StaticURLPath)
		}
		if PostsLocalDir != "posts" {
			t.Errorf("Expected PostsLocalDir 'posts', got %q", PostsLocalDir)
		}
		if PostsURLPath != "/posts/" {
			t.Errorf("Expected PostsURLPath '/posts/', got %q", PostsURLPath)
		}
		if TemplatesLocalDir != "templates" {
			t.Errorf("Expected TemplatesLocalDir 'templates', got %q", TemplatesLocalDir)
		}

		// Template names
		if TemplateLayout != "layout.html" {
			t.Errorf("Expected TemplateLayout 'layout.html', got %q", TemplateLayout)
		}
		if TemplateIndex != "index.html" {
			t.Errorf("Expected TemplateIndex 'index.html', got %q", TemplateIndex)
		}
		if TemplatePost != "post.html" {
			t.Errorf("Expected TemplatePost 'post.html', got %q", TemplatePost)
		}
		if TemplateEditor != "editor.html" {
			t.Errorf("Expected TemplateEditor 'editor.html', got %q", TemplateEditor)
		}
	})

	t.Run("HTTP constants", func(t *testing.T) {
		// Header constants
		if HCType != "Content-Type" {
			t.Errorf("Expected HCType 'Content-Type', got %q", HCType)
		}
		if HETag != "ETag" {
			t.Errorf("Expected HETag 'ETag', got %q", HETag)
		}
		if HCacheControl != "Cache-Control" {
			t.Errorf("Expected HCacheControl 'Cache-Control', got %q", HCacheControl)
		}
		if HHxRedirect != "Hx-Redirect" {
			t.Errorf("Expected HHxRedirect 'Hx-Redirect', got %q", HHxRedirect)
		}
		if HHxRefresh != "Hx-Refresh" {
			t.Errorf("Expected HHxRefresh 'Hx-Refresh', got %q", HHxRefresh)
		}

		// Content type constants
		if CTypeCSS != "text/css" {
			t.Errorf("Expected CTypeCSS 'text/css', got %q", CTypeCSS)
		}
		if CTypeHTML != "text/html" {
			t.Errorf("Expected CTypeHTML 'text/html', got %q", CTypeHTML)
		}
		if CTypeJSON != "application/json" {
			t.Errorf("Expected CTypeJSON 'application/json', got %q", CTypeJSON)
		}

		// Error constants
		if HTTPErrMethodNotAllowed != "Method not allowed" {
			t.Errorf("Expected HTTPErrMethodNotAllowed 'Method not allowed', got %q", HTTPErrMethodNotAllowed)
		}

		// Cookie constants
		if CookieTheme != "theme" {
			t.Errorf("Expected CookieTheme 'theme', got %q", CookieTheme)
		}
		if CookieSyntaxTheme != "syntax-theme" {
			t.Errorf("Expected CookieSyntaxTheme 'syntax-theme', got %q", CookieSyntaxTheme)
		}
		if CookieDraftID != "draft-id" {
			t.Errorf("Expected CookieDraftID 'draft-id', got %q", CookieDraftID)
		}
	})

	t.Run("Theme constants", func(t *testing.T) {
		if LightTheme != "light" {
			t.Errorf("Expected LightTheme 'light', got %q", LightTheme)
		}
		if DarkTheme != "dark" {
			t.Errorf("Expected DarkTheme 'dark', got %q", DarkTheme)
		}
		if LightThemeIcon != `<i class="fas fa-sun"></i>` {
			t.Errorf("Expected LightThemeIcon sun icon, got %q", LightThemeIcon)
		}
		if DarkThemeIcon != `<i class="fas fa-moon"></i>` {
			t.Errorf("Expected DarkThemeIcon moon icon, got %q", DarkThemeIcon)
		}
	})
}

func TestSliceDefaults(t *testing.T) {
	t.Run("Slice with whitespace handling", func(t *testing.T) {
		type TestStruct struct {
			Items []string `default:" item1 , item2 , item3 "`
		}

		test := &TestStruct{}
		applyDefaults(test)

		expected := []string{"item1", "item2", "item3"}
		if !reflect.DeepEqual(test.Items, expected) {
			t.Errorf("Expected trimmed items %v, got %v", expected, test.Items)
		}
	})

	t.Run("Empty slice default", func(t *testing.T) {
		type TestStruct struct {
			Items []string `default:""`
		}

		test := &TestStruct{}
		applyDefaults(test)

		// Empty string default is skipped, so slice remains nil/empty
		if test.Items != nil {
			t.Errorf("Expected nil slice for empty default, got %v", test.Items)
		}
	})

	t.Run("Single item slice", func(t *testing.T) {
		type TestStruct struct {
			Items []string `default:"single"`
		}

		test := &TestStruct{}
		applyDefaults(test)

		expected := []string{"single"}
		if !reflect.DeepEqual(test.Items, expected) {
			t.Errorf("Expected single item %v, got %v", expected, test.Items)
		}
	})

	t.Run("Non-empty slice should not be overwritten", func(t *testing.T) {
		type TestStruct struct {
			Items []string `default:"default1,default2"`
		}

		test := &TestStruct{Items: []string{"existing1", "existing2"}}
		applyDefaults(test)

		expected := []string{"existing1", "existing2"}
		if !reflect.DeepEqual(test.Items, expected) {
			t.Errorf("Expected existing items to be preserved %v, got %v", expected, test.Items)
		}
	})
}

func TestComplexNestedStructDefaults(t *testing.T) {
	// Test the actual Config struct with all its nested complexity
	config := &Config{}
	applyDefaults(config)

	// Verify deeply nested defaults work
	if config.Theme.SyntaxHighlighting.DefaultDark != "gruvbox" {
		t.Errorf("Expected nested default 'gruvbox', got %q", config.Theme.SyntaxHighlighting.DefaultDark)
	}

	// Verify all major sections have their defaults
	sections := []struct {
		name  string
		check func() bool
	}{
		{"Site", func() bool { return config.Site.Name != "" }},
		{"Server", func() bool { return config.Server.Host != "" }},
		{"Theme", func() bool { return config.Theme.Default != "" }},
		{"Content", func() bool { return config.Content.PostsPerPage > 0 }},
		{"Features", func() bool { return config.Features.Authentication.Type != "" }},
		{"Meta", func() bool { return len(config.Meta.Keywords) > 0 }},
		{"Logging", func() bool { return config.Logging.Level != "" }},
	}

	for _, section := range sections {
		if !section.check() {
			t.Errorf("Section %s defaults not applied correctly", section.name)
		}
	}
}
