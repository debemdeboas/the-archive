// Package config handles application configuration loading, validation, and default value management.
package config

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

var configLogger zerolog.Logger

func SetLogger(l zerolog.Logger) {
	configLogger = l
}

// Config represents the complete configuration structure
type Config struct {
	Version  string         `yaml:"version" default:"1.0" description:"Configuration version for migration compatibility"`
	Site     SiteConfig     `yaml:"site" description:"Site-specific settings like name and description"`
	Server   ServerConfig   `yaml:"server" description:"Server configuration including host and port"`
	Theme    ThemeConfig    `yaml:"theme" description:"Theme and visual customization options"`
	Posts    PostsConfig    `yaml:"posts" description:"Post display and reload configuration"`
	Features FeaturesConfig `yaml:"features" description:"Feature flags for optional functionality"`
	Meta     MetaConfig     `yaml:"meta" description:"HTML meta tags and SEO configuration"`
	Social   SocialConfig   `yaml:"social" description:"Social media links and contact information"`
	Logging  LoggingConfig  `yaml:"logging" description:"Logging level and output configuration"`
}

// LoggingConfig holds configuration for logging
type LoggingConfig struct {
	Level string `yaml:"level" default:"info" description:"Logging level: debug, info, warn, error" valid:"debug,info,warn,error"`
}

// SiteConfig holds site-specific configuration
type SiteConfig struct {
	Name        string `yaml:"name" default:"The Archive" description:"Site name displayed in title and header"`
	Description string `yaml:"description" default:"A personal blog and knowledge archive" description:"Site description for SEO and about sections"`
	Tagline     string `yaml:"tagline" default:"Welcome to The Archive" description:"Tagline displayed on the homepage"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Host string `yaml:"host" default:"0.0.0.0" description:"Server bind address (0.0.0.0 for all interfaces)"`
	Port string `yaml:"port" default:"12600" description:"Server port number"`
}

// ThemeConfig holds theme-related configuration
type ThemeConfig struct {
	Default            string       `yaml:"default" default:"dark" description:"Default theme" valid:"light,dark"`
	AllowSwitching     bool         `yaml:"allow_switching" default:"true" description:"Allow users to switch themes"`
	SyntaxHighlighting SyntaxConfig `yaml:"syntax_highlighting" description:"Syntax highlighting configuration"`
}

// SyntaxConfig holds syntax highlighting theme configuration
type SyntaxConfig struct {
	DefaultDark  string `yaml:"default_dark" default:"gruvbox" description:"Default syntax theme for dark mode"`
	DefaultLight string `yaml:"default_light" default:"catppuccin-latte" description:"Default syntax theme for light mode"`
}

// PostsConfig holds configuration related to posts display
type PostsConfig struct {
	ReloadTimeout int `yaml:"reload_timeout" default:"10" description:"How long to wait before reloading posts (in seconds)"`
	PostsPerPage  int `yaml:"posts_per_page" default:"50" description:"Number of posts to display per page"`
}

type FeaturesConfig struct {
	Authentication AuthConfig   `yaml:"authentication" description:"Authentication and security settings"`
	Editor         EditorConfig `yaml:"editor" description:"Post editor and creation features"`
	Search         FeatureFlag  `yaml:"search" description:"Enable search functionality"`
	Comments       FeatureFlag  `yaml:"comments" description:"Enable comment system"`
}

type AuthConfig struct {
	Enabled bool   `yaml:"enabled" default:"true" description:"Enable authentication system"`
	Type    string `yaml:"type" default:"ed25519" description:"Authentication type" valid:"ed25519"`
}

type EditorConfig struct {
	Enabled      bool `yaml:"enabled" default:"true" description:"Enable post editor interface"`
	LivePreview  bool `yaml:"live_preview" default:"true" description:"Enable live markdown preview"`
	EnableDrafts bool `yaml:"enable_drafts" default:"false" description:"Enable draft functionality"`
}

type FeatureFlag struct {
	Enabled bool `yaml:"enabled" default:"false" description:"Enable this feature"`
}

type MetaConfig struct {
	Author   string   `yaml:"author" default:"" description:"Site author name for meta tags"`
	Keywords []string `yaml:"keywords" default:"blog,archive,personal" description:"SEO keywords for meta tags"`
	Favicon  string   `yaml:"favicon" default:"/static/favicon.ico" description:"Path to favicon file"`
}

type SocialConfig struct {
	GitHub   string `yaml:"github" default:"" description:"GitHub profile URL"`
	Twitter  string `yaml:"twitter" default:"" description:"Twitter profile URL"`
	LinkedIn string `yaml:"linkedin" default:"" description:"LinkedIn profile URL"`
	Email    string `yaml:"email" default:"" description:"Contact email address"`
}

var AppConfig *Config

func LoadConfig(path string) error {
	config := &Config{}

	// Try to read and parse the config file
	data, err := os.ReadFile(path)
	if err != nil {
		// If file doesn't exist, create empty config and apply defaults
		configLogger.Info().Str("path", path).Msg("Config file not found, using defaults")
		applyDefaults(config)
		AppConfig = config
		return nil
	}

	// First, apply defaults to get a baseline
	applyDefaults(config)

	// Then unmarshal the YAML, which will override defaults where values are explicitly set
	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Migrate and validate configuration
	if err := MigrateConfig(config); err != nil {
		return fmt.Errorf("config migration failed: %w", err)
	}

	// Apply defaults one more time for any new fields added during migration
	applyDefaultsSelective(config, data)

	AppConfig = config
	configLogger.Info().Str("version", config.Version).Msg("Configuration loaded successfully")
	return nil
}

func ApplyDefaults(config any) {
	applyDefaults(config)
}

func applyDefaults(config any) {
	v := reflect.ValueOf(config)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		if !field.IsValid() || !field.CanSet() {
			continue
		}

		// Recursively apply defaults to nested structs
		if field.Kind() == reflect.Struct {
			applyDefaults(field.Addr().Interface())
			continue
		}

		defaultValue := fieldType.Tag.Get("default")
		if defaultValue == "" {
			continue
		}

		// Only apply default if field is zero value
		switch field.Kind() {
		case reflect.String:
			if field.String() == "" {
				field.SetString(defaultValue)
			}
		case reflect.Bool:
			if field.Bool() == false {
				if val, err := strconv.ParseBool(defaultValue); err == nil {
					field.SetBool(val)
				}
			}
		case reflect.Int:
			if field.Int() == 0 {
				if val, err := strconv.ParseInt(defaultValue, 10, 64); err == nil {
					field.SetInt(val)
				}
			}
		case reflect.Float64:
			if field.Float() == 0 {
				if val, err := strconv.ParseFloat(defaultValue, 64); err == nil {
					field.SetFloat(val)
				}
			}
		case reflect.Slice:
			if field.Len() == 0 && field.Type().Elem().Kind() == reflect.String {
				parts := strings.Split(defaultValue, ",")
				slice := reflect.MakeSlice(field.Type(), len(parts), len(parts))
				for j, part := range parts {
					slice.Index(j).SetString(strings.TrimSpace(part))
				}
				field.Set(slice)
			}
		default:
			configLogger.Warn().
				Str("field_name", fieldType.Name).
				Str("field_type", field.Kind().String()).
				Msg("Unsupported field type for default value")
		}
	}
}

// applyDefaultsSelective applies defaults only to fields not present in the original YAML
func applyDefaultsSelective(config any, originalYAML []byte) {
	// Parse the original YAML to see which fields were explicitly set
	var yamlMap map[string]any
	if err := yaml.Unmarshal(originalYAML, &yamlMap); err != nil {
		// If we can't parse the original YAML, just apply all defaults
		applyDefaults(config)
		return
	}

	applyDefaultsWithMap(config, yamlMap, "")
}

// applyDefaultsWithMap applies defaults while checking if fields exist in the YAML map
func applyDefaultsWithMap(config any, yamlMap map[string]any, prefix string) {
	v := reflect.ValueOf(config)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		if !field.IsValid() || !field.CanSet() {
			continue
		}

		yamlTag := fieldType.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue
		}

		yamlName := strings.Split(yamlTag, ",")[0]
		fieldPath := yamlName
		if prefix != "" {
			fieldPath = prefix + "." + yamlName
		}

		// Handle nested structs
		if field.Kind() == reflect.Struct {
			if nestedMap, ok := yamlMap[yamlName].(map[string]any); ok {
				applyDefaultsWithMap(field.Addr().Interface(), nestedMap, fieldPath)
			} else {
				// Field not present in YAML, apply defaults to entire nested struct
				applyDefaults(field.Addr().Interface())
			}
			continue
		}

		// Only apply default if field is not present in original YAML
		if _, exists := yamlMap[yamlName]; !exists {
			defaultValue := fieldType.Tag.Get("default")
			if defaultValue == "" {
				continue
			}

			switch field.Kind() {
			case reflect.String:
				field.SetString(defaultValue)
			case reflect.Bool:
				if val, err := strconv.ParseBool(defaultValue); err == nil {
					field.SetBool(val)
				}
			case reflect.Int:
				if val, err := strconv.ParseInt(defaultValue, 10, 64); err == nil {
					field.SetInt(val)
				}
			case reflect.Float64:
				if val, err := strconv.ParseFloat(defaultValue, 64); err == nil {
					field.SetFloat(val)
				}
			case reflect.Slice:
				if field.Type().Elem().Kind() == reflect.String {
					parts := strings.Split(defaultValue, ",")
					slice := reflect.MakeSlice(field.Type(), len(parts), len(parts))
					for j, part := range parts {
						slice.Index(j).SetString(strings.TrimSpace(part))
					}
					field.Set(slice)
				}
			}
		}
	}
}

// getGitCommitSHA returns the current git commit SHA, or a fallback if not available
func getGitCommitSHA() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		// Fallback if git is not available or we're not in a git repo
		return fmt.Sprintf("unknown-commit-%s", time.Now().Format("20060102-150405"))
	}
	
	commit := strings.TrimSpace(string(output))
	// Return short SHA (first 8 characters)
	if len(commit) >= 8 {
		return commit[:8]
	}
	return commit
}

// GenerateReference creates a comprehensive config reference with comments
func GenerateReference() ([]byte, error) {
	cfg := &Config{}
	applyDefaults(cfg)

	return generateYAMLWithComments(cfg)
}

// generateYAMLWithComments creates YAML with inline documentation
func generateYAMLWithComments(config any) ([]byte, error) {
	var result strings.Builder

	// Get current git commit SHA
	gitCommit := getGitCommitSHA()
	
	// Add header
	result.WriteString("# Configuration Reference for The Archive\n")
	result.WriteString(fmt.Sprintf("# Generated from commit: %s\n", gitCommit))
	result.WriteString("# This file shows all available configuration options with their defaults\n")
	result.WriteString("# Copy sections you want to customize to your config.yaml file\n\n")

	// Generate documented YAML
	if err := writeStructWithComments(&result, reflect.ValueOf(config), reflect.TypeOf(config), "", 0); err != nil {
		return nil, err
	}

	return []byte(result.String()), nil
}

// writeStructWithComments recursively writes struct fields with documentation
func writeStructWithComments(w *strings.Builder, v reflect.Value, t reflect.Type, prefix string, depth int) error {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
		t = t.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil
	}

	indent := strings.Repeat("  ", depth)

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		if !field.IsValid() || !field.CanInterface() {
			continue
		}

		yamlTag := fieldType.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue
		}

		yamlName := strings.Split(yamlTag, ",")[0]
		description := fieldType.Tag.Get("description")
		defaultValue := fieldType.Tag.Get("default")
		validValues := fieldType.Tag.Get("valid")

		// Write field documentation
		if description != "" {
			w.WriteString(fmt.Sprintf("%s# %s\n", indent, description))
		}
		if defaultValue != "" {
			w.WriteString(fmt.Sprintf("%s# Default: %s\n", indent, defaultValue))
		}
		if validValues != "" {
			w.WriteString(fmt.Sprintf("%s# Valid values: %s\n", indent, validValues))
		}

		// Handle nested structs
		if field.Kind() == reflect.Struct {
			w.WriteString(fmt.Sprintf("%s%s:\n", indent, yamlName))
			if err := writeStructWithComments(w, field, fieldType.Type, prefix+yamlName+".", depth+1); err != nil {
				return err
			}
		} else {
			// Write field with value
			value := formatFieldValue(field)
			w.WriteString(fmt.Sprintf("%s%s: %s\n", indent, yamlName, value))
		}

		if i < v.NumField()-1 {
			w.WriteString("\n")
		}
	}

	return nil
}

// formatFieldValue formats a field value for YAML output
func formatFieldValue(field reflect.Value) string {
	switch field.Kind() {
	case reflect.String:
		if field.String() == "" {
			return `""`
		}
		return fmt.Sprintf(`"%s"`, field.String())
	case reflect.Bool:
		return fmt.Sprintf("%t", field.Bool())
	case reflect.Int, reflect.Int64:
		return fmt.Sprintf("%d", field.Int())
	case reflect.Float64:
		return fmt.Sprintf("%.2f", field.Float())
	case reflect.Slice:
		if field.Len() == 0 {
			return "[]"
		}
		var items []string
		for i := 0; i < field.Len(); i++ {
			elem := field.Index(i)
			if elem.Kind() == reflect.String {
				items = append(items, fmt.Sprintf(`"%s"`, elem.String()))
			} else {
				items = append(items, fmt.Sprintf("%v", elem.Interface()))
			}
		}
		return fmt.Sprintf("[%s]", strings.Join(items, ", "))
	default:
		return fmt.Sprintf("%v", field.Interface())
	}
}

// ValidateVersion checks if the configuration version is supported
func ValidateVersion(version string) error {
	supportedVersions := []string{"1.0"}

	for _, supported := range supportedVersions {
		if version == supported {
			return nil
		}
	}

	return fmt.Errorf("unsupported configuration version: %s (supported: %v)", version, supportedVersions)
}

// MigrateConfig migrates configuration from older versions
func MigrateConfig(config *Config) error {
	if config.Version == "" {
		config.Version = "1.0" // Set default version for legacy configs
	}

	if err := ValidateVersion(config.Version); err != nil {
		return err
	}

	// Future: Add migration logic for different versions
	switch config.Version {
	case "1.0":
		// Current version, no migration needed
		return nil
	default:
		return fmt.Errorf("unknown configuration version: %s", config.Version)
	}
}
