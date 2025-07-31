package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

var configLogger zerolog.Logger

func SetLogger(l zerolog.Logger) {
	configLogger = l
}

// Config represents the complete configuration structure
type Config struct {
	Site     SiteConfig     `yaml:"site"`
	Server   ServerConfig   `yaml:"server"`
	Theme    ThemeConfig    `yaml:"theme"`
	Content  ContentConfig  `yaml:"content"`
	Features FeaturesConfig `yaml:"features"`
	Meta     MetaConfig     `yaml:"meta"`
	Social   SocialConfig   `yaml:"social"`
	Logging  LoggingConfig  `yaml:"logging"`
}

type LoggingConfig struct {
	Level string `yaml:"level" default:"info"`
}

type SiteConfig struct {
	Name        string `yaml:"name" default:"The Archive"`
	Description string `yaml:"description" default:"A personal blog and knowledge archive"`
	Tagline     string `yaml:"tagline" default:"Welcome to The Archive"`
}

type ServerConfig struct {
	Host string `yaml:"host" default:"0.0.0.0"`
	Port string `yaml:"port" default:"12600"`
}

type ThemeConfig struct {
	Default            string       `yaml:"default" default:"dark"`
	AllowSwitching     bool         `yaml:"allow_switching" default:"true"`
	SyntaxHighlighting SyntaxConfig `yaml:"syntax_highlighting"`
}

type SyntaxConfig struct {
	DefaultDark  string `yaml:"default_dark" default:"gruvbox"`
	DefaultLight string `yaml:"default_light" default:"catppuccin-latte"`
}

type ContentConfig struct {
	PostsPerPage int `yaml:"posts_per_page" default:"50"`
}

type FeaturesConfig struct {
	Authentication AuthConfig   `yaml:"authentication"`
	Editor         EditorConfig `yaml:"editor"`
	Search         FeatureFlag  `yaml:"search"`
	Comments       FeatureFlag  `yaml:"comments"`
}

type AuthConfig struct {
	Enabled bool   `yaml:"enabled" default:"true"`
	Type    string `yaml:"type" default:"ed25519"`
}

type EditorConfig struct {
	Enabled     bool `yaml:"enabled" default:"true"`
	LivePreview bool `yaml:"live_preview" default:"true"`
}

type FeatureFlag struct {
	Enabled bool `yaml:"enabled" default:"false"`
}

type MetaConfig struct {
	Author   string   `yaml:"author" default:""`
	Keywords []string `yaml:"keywords" default:"blog,archive,personal"`
	Favicon  string   `yaml:"favicon" default:"/static/favicon.ico"`
}

type SocialConfig struct {
	GitHub   string `yaml:"github" default:""`
	Twitter  string `yaml:"twitter" default:""`
	LinkedIn string `yaml:"linkedin" default:""`
	Email    string `yaml:"email" default:""`
}

var AppConfig *Config

func LoadConfig(path string) error {
	config := &Config{}

	// Apply default values first
	applyDefaults(config)

	// Try to read and parse the config file
	data, err := os.ReadFile(path)
	if err != nil {
		// If file doesn't exist, just use defaults
		configLogger.Info().Str("path", path).Msg("Config file not found, using defaults")
		AppConfig = config
		return nil
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	AppConfig = config
	return nil
}

func ApplyDefaults(config interface{}) {
	applyDefaults(config)
}

func applyDefaults(config interface{}) {
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
