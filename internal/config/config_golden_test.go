package config

import (
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// TestConfigDefaultsGoldenFile tests that our defaults match the golden file
func TestConfigDefaultsGoldenFile(t *testing.T) {
	// Set up logger for testing
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	SetLogger(logger)

	// Load the golden defaults file
	goldenData, err := os.ReadFile("testdata/defaults.yaml")
	if err != nil {
		t.Fatalf("Failed to read golden defaults file: %v", err)
	}

	// Parse golden config
	var goldenConfig Config
	if err := yaml.Unmarshal(goldenData, &goldenConfig); err != nil {
		t.Fatalf("Failed to parse golden config: %v", err)
	}

	// Create a new config with defaults applied
	testConfig := &Config{}
	ApplyDefaults(testConfig)

	// Compare key fields
	if testConfig.Version != goldenConfig.Version {
		t.Errorf("Version mismatch: got %q, want %q", testConfig.Version, goldenConfig.Version)
	}
	if testConfig.Site.Name != goldenConfig.Site.Name {
		t.Errorf("Site.Name mismatch: got %q, want %q", testConfig.Site.Name, goldenConfig.Site.Name)
	}
	if testConfig.Server.Port != goldenConfig.Server.Port {
		t.Errorf("Server.Port mismatch: got %q, want %q", testConfig.Server.Port, goldenConfig.Server.Port)
	}
	if testConfig.Theme.Default != goldenConfig.Theme.Default {
		t.Errorf("Theme.Default mismatch: got %q, want %q", testConfig.Theme.Default, goldenConfig.Theme.Default)
	}
	if testConfig.Features.Authentication.Enabled != goldenConfig.Features.Authentication.Enabled {
		t.Errorf("Features.Authentication.Enabled mismatch: got %v, want %v",
			testConfig.Features.Authentication.Enabled, goldenConfig.Features.Authentication.Enabled)
	}
}

// TestConfigConstantsMatch tests that generated constants match actual defaults
func TestConfigConstantsMatch(t *testing.T) {
	cfg := &Config{}
	ApplyDefaults(cfg)

	// Test against generated constants
	if cfg.Version != DefaultVersion {
		t.Errorf("Version constant mismatch: got %q, want %q", cfg.Version, DefaultVersion)
	}
	if cfg.Site.Name != DefaultSiteName {
		t.Errorf("Site.Name constant mismatch: got %q, want %q", cfg.Site.Name, DefaultSiteName)
	}
	if cfg.Server.Host != DefaultServerHost {
		t.Errorf("Server.Host constant mismatch: got %q, want %q", cfg.Server.Host, DefaultServerHost)
	}
	if cfg.Theme.AllowSwitching != DefaultThemeAllowSwitching {
		t.Errorf("Theme.AllowSwitching constant mismatch: got %v, want %v",
			cfg.Theme.AllowSwitching, DefaultThemeAllowSwitching)
	}
}

// TestInvalidConfigValidation tests validation using generated invalid configs
func TestInvalidConfigValidation(t *testing.T) {
	logger := zerolog.New(os.Stdout).Level(zerolog.ErrorLevel)
	SetLogger(logger)

	testCases := []struct {
		name        string
		filename    string
		expectError bool
		errorText   string
	}{
		{
			name:        "Invalid version",
			filename:    "testdata/invalid_version.yaml",
			expectError: true,
			errorText:   "unsupported configuration version",
		},
		{
			name:        "Valid defaults file",
			filename:    "testdata/defaults.yaml",
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			originalAppConfig := AppConfig
			defer func() { AppConfig = originalAppConfig }()

			err := LoadConfig(tc.filename)

			if tc.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if tc.expectError && err != nil && tc.errorText != "" {
				if !containsString(err.Error(), tc.errorText) {
					t.Errorf("Expected error to contain %q, got %q", tc.errorText, err.Error())
				}
			}
		})
	}
}

// TestGeneratedFilesUpToDate ensures generated files are current
func TestGeneratedFilesUpToDate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping generated files check in short mode")
	}

	// Check if defaults.yaml exists and is recent
	_, err := os.Stat("testdata/defaults.yaml")
	if err != nil {
		t.Errorf("Generated defaults.yaml not found. Run 'make config-update-tests' to generate.")
		return
	}

	// Check if constants file exists
	if _, err := os.Stat("config_generated_constants.go"); err != nil {
		t.Errorf("Generated config_generated_constants.go not found. Run 'make config-update-tests' to generate.")
	}
}

// containsString checks if a string contains a substring (helper function)
func containsString(s, substr string) bool {
	return len(substr) <= len(s) && (substr == "" || strings.Contains(s, substr))
}
