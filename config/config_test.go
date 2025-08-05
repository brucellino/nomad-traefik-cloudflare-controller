package config

// Unit tests for the config package.

import (
	"os"
	"testing"
)

// The GetEnvOrDefault function should set defaults for required environment variables if they are not set
func TestGetEnvOrDefault(t *testing.T) {
	// We define a map of test cases which have a set of attributes.
	tests := []struct {
		name         string
		envKey       string
		envValue     string
		defaultValue string
		expected     string
	}{
		{
			name:         "When the environment variable is set, return the environment variable value.",
			envKey:       "TEST_KEY",
			envValue:     "test_value",
			defaultValue: "default",
			expected:     "test_value",
		},
		{
			name:         "When the environment variable is not set, return the default value.",
			envKey:       "UNSET_KEY",
			envValue:     "",
			defaultValue: "default",
			expected:     "default",
		},
	}

	// Loop over set of test cases, for each test case set an environment variable if the test case sets it,
	// else keep it clean.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing env var
			os.Unsetenv(tt.envKey)

			// Set env var if needed
			if tt.envValue != "" {
				os.Setenv(tt.envKey, tt.envValue)
				defer os.Unsetenv(tt.envKey)
			}

			result := getEnvOrDefault(tt.envKey, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("getEnvOrDefault(%q, %q) = %q, want %q", tt.envKey, tt.defaultValue, result, tt.expected)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// tests is a list of test scenarios
	// Each scenario has a name, a list of environment variables to be set in the scenario,
	// whether or not to expect an error
	// and an associated error message
	tests := []struct {
		name        string
		envVars     map[string]string
		expectError bool
		errorMsg    string
	}{
		{
			// We should be able to set all variables explicitly and create a valid configuration.
			name: "Valid configuration with all required fields should give no errors.",
			envVars: map[string]string{
				"CLOUDFLARE_API_TOKEN": "test_token",
				"CLOUDFLARE_ZONE_ID":   "test_zone_id",
				"NOMAD_TOKEN":          "test_nomad_token",
				"NOMAD_ADDR":           "http://test:4646",
				"TRAEFIK_JOB_NAME":     "traefik",
				"DNS_RECORD_NAME":      "test.example.com",
				"LOG_LEVEL":            "debug",
			},
			expectError: false,
		},
		{
			// For a valid configuration which does not explicitly set variables for which there are defaults, use the defaults.
			// This scenario should not give an error.
			name: "Valid configuration with for variables which have no defaults should use defaults for the rest and give no errors.",
			envVars: map[string]string{
				"CLOUDFLARE_API_TOKEN": "test_token",
				"CLOUDFLARE_ZONE_ID":   "test_zone_id",
				"NOMAD_TOKEN":          "test_nomad_token",
			},
			expectError: false,
		},
		{
			name: "Missing cloudflare token is an invalid configuration, because there is no default.",
			envVars: map[string]string{
				"CLOUDFLARE_ZONE_ID": "test_zone_id",
				"NOMAD_TOKEN":        "test_nomad_token",
			},
			expectError: true,
			errorMsg:    "CLOUDFLARE_API_TOKEN is not set and is required.",
		},
		{
			name: "Missing cloudflare zone id is an invalid configuration since there is no default",
			envVars: map[string]string{
				"CLOUDFLARE_API_TOKEN": "test_token",
				"NOMAD_TOKEN":          "test_nomad_token",
			},
			expectError: true,
			errorMsg:    "CLOUDFLARE_ZONE_ID is not set and is required.",
		},
		{
			name: "Missing Nomad token is an invalid configuration since there is no default.",
			envVars: map[string]string{
				"CLOUDFLARE_API_TOKEN": "test_token",
				"CLOUDFLARE_ZONE_ID":   "test_zone_id",
			},
			expectError: true,
			errorMsg:    "Nomad token is not set and is required.",
		},
	}

	// Loop over the test cases, setup the
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all relevant environment variables
			envKeys := []string{
				"NOMAD_ADDR", "NOMAD_TOKEN", "CLOUDFLARE_API_TOKEN",
				"CLOUDFLARE_ZONE_ID", "TRAEFIK_JOB_NAME", "DNS_RECORD_NAME", "LOG_LEVEL",
			}
			// For each key, unset it so that we revert to defaults
			for _, key := range envKeys {
				os.Unsetenv(key)
			}

			// For each environment variable which the test case does set, set the environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Clean up after test
			defer func() {
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			// Create scenario by loading the config.
			config, err := LoadConfig()

			if tt.expectError {
				if err == nil {
					t.Errorf("LoadConfig() expected error but got none")
					return
				}
				if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("LoadConfig() error = %q, want %q", err.Error(), tt.errorMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("LoadConfig() unexpected error = %v", err)
				return
			}

			// Verify default values
			if config.NomadAddress == "" {
				t.Error("NomadAddress should have default value")
			}
			if config.LogLevel == "" {
				t.Error("LogLevel should have default value")
			}

			// Verify required fields are set
			if config.CloudflareToken == "" {
				t.Error("CloudflareToken should be set")
			}
			if config.CloudflareZoneId == "" {
				t.Error("CloudflareZoneId should be set")
			}
			if config.NomadToken == "" {
				t.Error("NomadToken should be set")
			}

			// Test specific values if provided
			if nomadAddr, exists := tt.envVars["NOMAD_ADDR"]; exists && config.NomadAddress != nomadAddr {
				t.Errorf("NomadAddress = %q, want %q", config.NomadAddress, nomadAddr)
			}
			if logLevel, exists := tt.envVars["LOG_LEVEL"]; exists && config.LogLevel != logLevel {
				t.Errorf("LogLevel = %q, want %q", config.LogLevel, logLevel)
			}
		})
	}
}

// TestLoadConfigDefaults tests the default values of the configuration.
func TestLoadConfigDefaults(t *testing.T) {
	// Clear environment
	envKeys := []string{
		"NOMAD_ADDR", "NOMAD_TOKEN", "CLOUDFLARE_API_TOKEN",
		"CLOUDFLARE_ZONE_ID", "TRAEFIK_JOB_NAME", "DNS_RECORD_NAME", "LOG_LEVEL",
	}
	for _, key := range envKeys {
		os.Unsetenv(key)
	}
	defer func() {
		for _, key := range envKeys {
			os.Unsetenv(key)
		}
	}()

	// Set only required fields
	os.Setenv("CLOUDFLARE_API_TOKEN", "test_token")
	os.Setenv("CLOUDFLARE_ZONE_ID", "test_zone_id")
	os.Setenv("NOMAD_TOKEN", "test_nomad_token")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Test default values
	expectedDefaults := map[string]string{
		"NomadAddress": "http://localhost:8686",
		"LogLevel":     "info",
	}

	if config.NomadAddress != expectedDefaults["NomadAddress"] {
		t.Errorf("NomadAddress default = %q, want %q", config.NomadAddress, expectedDefaults["NomadAddress"])
	}
	if config.LogLevel != expectedDefaults["LogLevel"] {
		t.Errorf("LogLevel default = %q, want %q", config.LogLevel, expectedDefaults["LogLevel"])
	}
}
