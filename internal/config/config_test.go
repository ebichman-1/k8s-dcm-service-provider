package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected *Config
	}{
		{
			name:    "default configuration",
			envVars: map[string]string{},
			expected: &Config{
				Server: ServerConfig{
					Port:         8080,
					Host:         "0.0.0.0",
					ReadTimeout:  30,
					WriteTimeout: 30,
				},
				Kubernetes: KubernetesConfig{
					ConfigPath: "",
					InCluster:  false,
				},
				Log: LogConfig{
					Level:      "info",
					Format:     "json",
					OutputPath: "stdout",
				},
			},
		},
		{
			name: "custom configuration",
			envVars: map[string]string{
				"SERVER_PORT":         "9090",
				"SERVER_HOST":         "127.0.0.1",
				"SERVER_READ_TIMEOUT": "60",
				"IN_CLUSTER":          "true",
				"LOG_LEVEL":           "debug",
				"LOG_FORMAT":          "console",
			},
			expected: &Config{
				Server: ServerConfig{
					Port:         9090,
					Host:         "127.0.0.1",
					ReadTimeout:  60,
					WriteTimeout: 30,
				},
				Kubernetes: KubernetesConfig{
					ConfigPath: "",
					InCluster:  true,
				},
				Log: LogConfig{
					Level:      "debug",
					Format:     "console",
					OutputPath: "stdout",
				},
			},
		},
		{
			name: "kubeconfig path set",
			envVars: map[string]string{
				"KUBECONFIG":      "/custom/kubeconfig",
				"LOG_LEVEL":       "error",
				"LOG_OUTPUT_PATH": "/var/log/app.log",
			},
			expected: &Config{
				Server: ServerConfig{
					Port:         8080,
					Host:         "0.0.0.0",
					ReadTimeout:  30,
					WriteTimeout: 30,
				},
				Kubernetes: KubernetesConfig{
					ConfigPath: "/custom/kubeconfig",
					InCluster:  false,
				},
				Log: LogConfig{
					Level:      "error",
					Format:     "json",
					OutputPath: "/var/log/app.log",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment variables
			envVarsToClean := []string{
				"SERVER_PORT", "SERVER_HOST", "SERVER_READ_TIMEOUT", "SERVER_WRITE_TIMEOUT",
				"KUBECONFIG", "IN_CLUSTER", "LOG_LEVEL", "LOG_FORMAT", "LOG_OUTPUT_PATH",
			}
			for _, envVar := range envVarsToClean {
				_ = os.Unsetenv(envVar)
			}

			// Set test environment variables
			for key, value := range tt.envVars {
				if err := os.Setenv(key, value); err != nil {
					t.Fatalf("Failed to set environment variable %s: %v", key, err)
				}
			}

			// Load configuration
			config := LoadConfig()

			// Assert expectations
			assert.Equal(t, tt.expected.Server.Port, config.Server.Port)
			assert.Equal(t, tt.expected.Server.Host, config.Server.Host)
			assert.Equal(t, tt.expected.Server.ReadTimeout, config.Server.ReadTimeout)
			assert.Equal(t, tt.expected.Server.WriteTimeout, config.Server.WriteTimeout)
			assert.Equal(t, tt.expected.Kubernetes.ConfigPath, config.Kubernetes.ConfigPath)
			assert.Equal(t, tt.expected.Kubernetes.InCluster, config.Kubernetes.InCluster)
			assert.Equal(t, tt.expected.Log.Level, config.Log.Level)
			assert.Equal(t, tt.expected.Log.Format, config.Log.Format)
			assert.Equal(t, tt.expected.Log.OutputPath, config.Log.OutputPath)

			// Clean up environment variables
			for key := range tt.envVars {
				_ = os.Unsetenv(key)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		fallback string
		envValue string
		expected string
	}{
		{
			name:     "environment variable exists",
			key:      "TEST_VAR",
			fallback: "default",
			envValue: "custom_value",
			expected: "custom_value",
		},
		{
			name:     "environment variable does not exist",
			key:      "NON_EXISTENT_VAR",
			fallback: "default",
			envValue: "",
			expected: "default",
		},
		{
			name:     "empty environment variable",
			key:      "EMPTY_VAR",
			fallback: "default",
			envValue: "",
			expected: "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up environment variable
			_ = os.Unsetenv(tt.key)

			// Set environment variable if needed
			if tt.envValue != "" {
				if err := os.Setenv(tt.key, tt.envValue); err != nil {
					t.Fatalf("Failed to set environment variable %s: %v", tt.key, err)
				}
			}

			// Test function
			result := getEnv(tt.key, tt.fallback)
			assert.Equal(t, tt.expected, result)

			// Clean up
			_ = os.Unsetenv(tt.key)
		})
	}
}

func TestGetEnvAsInt(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		fallback int
		envValue string
		expected int
	}{
		{
			name:     "valid integer",
			key:      "TEST_INT",
			fallback: 100,
			envValue: "200",
			expected: 200,
		},
		{
			name:     "invalid integer",
			key:      "TEST_INT_INVALID",
			fallback: 100,
			envValue: "not_a_number",
			expected: 100,
		},
		{
			name:     "empty environment variable",
			key:      "TEST_INT_EMPTY",
			fallback: 100,
			envValue: "",
			expected: 100,
		},
		{
			name:     "zero value",
			key:      "TEST_INT_ZERO",
			fallback: 100,
			envValue: "0",
			expected: 0,
		},
		{
			name:     "negative value",
			key:      "TEST_INT_NEGATIVE",
			fallback: 100,
			envValue: "-50",
			expected: -50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up environment variable
			_ = os.Unsetenv(tt.key)

			// Set environment variable if needed
			if tt.envValue != "" {
				if err := os.Setenv(tt.key, tt.envValue); err != nil {
					t.Fatalf("Failed to set environment variable %s: %v", tt.key, err)
				}
			}

			// Test function
			result := getEnvAsInt(tt.key, tt.fallback)
			assert.Equal(t, tt.expected, result)

			// Clean up
			_ = os.Unsetenv(tt.key)
		})
	}
}

func TestGetEnvAsBool(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		fallback bool
		envValue string
		expected bool
	}{
		{
			name:     "true value",
			key:      "TEST_BOOL_TRUE",
			fallback: false,
			envValue: "true",
			expected: true,
		},
		{
			name:     "false value",
			key:      "TEST_BOOL_FALSE",
			fallback: true,
			envValue: "false",
			expected: false,
		},
		{
			name:     "1 value",
			key:      "TEST_BOOL_ONE",
			fallback: false,
			envValue: "1",
			expected: true,
		},
		{
			name:     "0 value",
			key:      "TEST_BOOL_ZERO",
			fallback: true,
			envValue: "0",
			expected: false,
		},
		{
			name:     "invalid boolean",
			key:      "TEST_BOOL_INVALID",
			fallback: true,
			envValue: "not_a_bool",
			expected: true,
		},
		{
			name:     "empty environment variable",
			key:      "TEST_BOOL_EMPTY",
			fallback: true,
			envValue: "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up environment variable
			_ = os.Unsetenv(tt.key)

			// Set environment variable if needed
			if tt.envValue != "" {
				if err := os.Setenv(tt.key, tt.envValue); err != nil {
					t.Fatalf("Failed to set environment variable %s: %v", tt.key, err)
				}
			}

			// Test function
			result := getEnvAsBool(tt.key, tt.fallback)
			assert.Equal(t, tt.expected, result)

			// Clean up
			_ = os.Unsetenv(tt.key)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid configuration",
			config: &Config{
				Server: ServerConfig{
					Port: 8080,
					Host: "0.0.0.0",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid port - too low",
			config: &Config{
				Server: ServerConfig{
					Port: 0,
					Host: "0.0.0.0",
				},
			},
			wantErr: true,
			errMsg:  "invalid server port",
		},
		{
			name: "invalid port - too high",
			config: &Config{
				Server: ServerConfig{
					Port: 70000,
					Host: "0.0.0.0",
				},
			},
			wantErr: true,
			errMsg:  "invalid server port",
		},
		{
			name: "empty host",
			config: &Config{
				Server: ServerConfig{
					Port: 8080,
					Host: "",
				},
			},
			wantErr: true,
			errMsg:  "server host cannot be empty",
		},
		{
			name: "valid port boundary - minimum",
			config: &Config{
				Server: ServerConfig{
					Port: 1,
					Host: "localhost",
				},
			},
			wantErr: false,
		},
		{
			name: "valid port boundary - maximum",
			config: &Config{
				Server: ServerConfig{
					Port: 65535,
					Host: "localhost",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigIntegration(t *testing.T) {
	// Test that a fully loaded configuration can be validated
	originalEnvVars := map[string]string{
		"SERVER_PORT":         os.Getenv("SERVER_PORT"),
		"SERVER_HOST":         os.Getenv("SERVER_HOST"),
		"SERVER_READ_TIMEOUT": os.Getenv("SERVER_READ_TIMEOUT"),
		"KUBECONFIG":          os.Getenv("KUBECONFIG"),
		"IN_CLUSTER":          os.Getenv("IN_CLUSTER"),
		"LOG_LEVEL":           os.Getenv("LOG_LEVEL"),
	}

	// Set test environment
	testEnvVars := map[string]string{
		"SERVER_PORT":         "9000",
		"SERVER_HOST":         "127.0.0.1",
		"SERVER_READ_TIMEOUT": "45",
		"KUBECONFIG":          "/test/kubeconfig",
		"IN_CLUSTER":          "false",
		"LOG_LEVEL":           "warn",
	}

	for key, value := range testEnvVars {
		if err := os.Setenv(key, value); err != nil {
			t.Fatalf("Failed to set test environment variable %s: %v", key, err)
		}
	}

	// Load and validate configuration
	config := LoadConfig()
	err := config.Validate()

	assert.NoError(t, err)
	assert.Equal(t, 9000, config.Server.Port)
	assert.Equal(t, "127.0.0.1", config.Server.Host)
	assert.Equal(t, 45, config.Server.ReadTimeout)
	assert.Equal(t, "/test/kubeconfig", config.Kubernetes.ConfigPath)
	assert.Equal(t, false, config.Kubernetes.InCluster)
	assert.Equal(t, "warn", config.Log.Level)

	// Restore original environment
	for key, value := range originalEnvVars {
		if value == "" {
			_ = os.Unsetenv(key)
		} else {
			if err := os.Setenv(key, value); err != nil {
				t.Errorf("Failed to restore environment variable %s: %v", key, err)
			}
		}
	}
}
