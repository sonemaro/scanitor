package config

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	// Helper function to clean environment variables after each test
	cleanup := func() {
		envVars := []string{
			"SCANITOR_WORKERS",
			"SCANITOR_MAX_DEPTH",
			"SCANITOR_IGNORE",
			"SCANITOR_OUTPUT",
			"SCANITOR_OUTPUT_FILE",
			"SCANITOR_RATE_LIMIT",
			"SCANITOR_BUFFER_SIZE",
			"SCANITOR_NO_PROGRESS",
			"SCANITOR_NO_COLOR",
			"SCANITOR_VERBOSE",
		}
		for _, env := range envVars {
			os.Unsetenv(env)
		}
	}

	tests := []struct {
		name     string
		envVars  map[string]string
		expected Config
		wantErr  bool
		errMsg   string
	}{
		{
			name: "default configuration",
			expected: Config{
				Workers:    runtime.NumCPU(),
				MaxDepth:   -1,
				BufferSize: 4096,
				Output:     "tree",
				Verbose:    0,
				NoProgress: false,
				NoColor:    false,
				RateLimit:  0,
			},
		},
		{
			name: "configuration from environment variables",
			envVars: map[string]string{
				"SCANITOR_WORKERS":     "4",
				"SCANITOR_MAX_DEPTH":   "10",
				"SCANITOR_IGNORE":      "node_modules,.git,*.tmp",
				"SCANITOR_OUTPUT":      "json",
				"SCANITOR_OUTPUT_FILE": "output.json",
				"SCANITOR_RATE_LIMIT":  "100",
				"SCANITOR_BUFFER_SIZE": "8192",
				"SCANITOR_NO_PROGRESS": "true",
				"SCANITOR_NO_COLOR":    "true",
				"SCANITOR_VERBOSE":     "vv",
			},
			expected: Config{
				Workers:        4,
				MaxDepth:       10,
				IgnorePatterns: []string{"node_modules", ".git", "*.tmp"},
				Output:         "json",
				OutputFile:     "output.json",
				RateLimit:      100,
				BufferSize:     8192,
				NoProgress:     true,
				NoColor:        true,
				Verbose:        2,
			},
		},
		{
			name: "invalid workers count - negative",
			envVars: map[string]string{
				"SCANITOR_WORKERS": "-1",
			},
			wantErr: true,
			errMsg:  "workers count must be positive",
		},
		{
			name: "invalid workers count - zero",
			envVars: map[string]string{
				"SCANITOR_WORKERS": "0",
			},
			expected: Config{
				Workers:    runtime.NumCPU(), // Should default to NumCPU
				MaxDepth:   -1,
				Output:     "tree",
				BufferSize: 4096,
			},
		},
		{
			name: "invalid output format",
			envVars: map[string]string{
				"SCANITOR_OUTPUT": "invalid",
			},
			wantErr: true,
			errMsg:  "invalid output format: must be one of [tree json yaml]",
		},
		{
			name: "invalid buffer size - negative",
			envVars: map[string]string{
				"SCANITOR_BUFFER_SIZE": "-1",
			},
			wantErr: true,
			errMsg:  "buffer size must be positive",
		},
		{
			name: "invalid buffer size - too small",
			envVars: map[string]string{
				"SCANITOR_BUFFER_SIZE": "63",
			},
			wantErr: true,
			errMsg:  "buffer size must be at least 64 bytes",
		},
		{
			name: "invalid max depth - negative but not -1",
			envVars: map[string]string{
				"SCANITOR_MAX_DEPTH": "-2",
			},
			wantErr: true,
			errMsg:  "max depth must be -1 (unlimited) or positive",
		},
		{
			name: "invalid rate limit - negative",
			envVars: map[string]string{
				"SCANITOR_RATE_LIMIT": "-1",
			},
			wantErr: true,
			errMsg:  "rate limit must be non-negative",
		},
		{
			name: "empty ignore patterns",
			envVars: map[string]string{
				"SCANITOR_IGNORE": "",
			},
			expected: Config{
				Workers:        runtime.NumCPU(),
				MaxDepth:       -1,
				IgnorePatterns: []string{},
				Output:         "tree",
				BufferSize:     4096,
			},
		},
		{
			name: "multiple verbosity levels",
			envVars: map[string]string{
				"SCANITOR_VERBOSE": "vvv",
			},
			expected: Config{
				Workers:    runtime.NumCPU(),
				MaxDepth:   -1,
				Output:     "tree",
				BufferSize: 4096,
				Verbose:    3,
			},
		},
		{
			name: "boolean parsing - various true values",
			envVars: map[string]string{
				"SCANITOR_NO_PROGRESS": "true",
				"SCANITOR_NO_COLOR":    "1",
			},
			expected: Config{
				Workers:    runtime.NumCPU(),
				MaxDepth:   -1,
				Output:     "tree",
				BufferSize: 4096,
				NoProgress: true,
				NoColor:    true,
			},
		},
		{
			name: "boolean parsing - various false values",
			envVars: map[string]string{
				"SCANITOR_NO_PROGRESS": "false",
				"SCANITOR_NO_COLOR":    "0",
			},
			expected: Config{
				Workers:    runtime.NumCPU(),
				MaxDepth:   -1,
				Output:     "tree",
				BufferSize: 4096,
				NoProgress: false,
				NoColor:    false,
			},
		},
		{
			name: "ignore patterns with spaces",
			envVars: map[string]string{
				"SCANITOR_IGNORE": "node_modules, .git, *.tmp",
			},
			expected: Config{
				Workers:        runtime.NumCPU(),
				MaxDepth:       -1,
				IgnorePatterns: []string{"node_modules", ".git", "*.tmp"},
				Output:         "tree",
				BufferSize:     4096,
			},
		},
		{
			name: "maximum workers limit",
			envVars: map[string]string{
				"SCANITOR_WORKERS": "1000000",
			},
			wantErr: true,
			errMsg:  "workers count cannot exceed system CPU count * 4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean environment before each test
			cleanup()

			// Set environment variables for test
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}

			// Load configuration
			cfg, err := Load()

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected.Workers, cfg.Workers)
			assert.Equal(t, tt.expected.MaxDepth, cfg.MaxDepth)

			// Compare ignore patterns after trimming spaces
			if len(tt.expected.IgnorePatterns) > 0 {
				expectedPatterns := make([]string, len(tt.expected.IgnorePatterns))
				actualPatterns := make([]string, len(cfg.IgnorePatterns))

				for i, p := range tt.expected.IgnorePatterns {
					expectedPatterns[i] = strings.TrimSpace(p)
				}
				for i, p := range cfg.IgnorePatterns {
					actualPatterns[i] = strings.TrimSpace(p)
				}
				assert.Equal(t, expectedPatterns, actualPatterns)
			}

			assert.Equal(t, tt.expected.Output, cfg.Output)
			assert.Equal(t, tt.expected.OutputFile, cfg.OutputFile)
			assert.Equal(t, tt.expected.RateLimit, cfg.RateLimit)
			assert.Equal(t, tt.expected.BufferSize, cfg.BufferSize)
			assert.Equal(t, tt.expected.NoProgress, cfg.NoProgress)
			assert.Equal(t, tt.expected.NoColor, cfg.NoColor)
			assert.Equal(t, tt.expected.Verbose, cfg.Verbose)
		})
	}
}

func TestValidateConfig(t *testing.T) {
	maxWorkers := runtime.NumCPU() * 4

	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid configuration",
			config: Config{
				Workers:    4,
				MaxDepth:   10,
				Output:     "json",
				BufferSize: 4096,
			},
			wantErr: false,
		},
		{
			name: "invalid workers count - negative",
			config: Config{
				Workers:    -1,
				Output:     "json",
				BufferSize: 4096,
			},
			wantErr: true,
			errMsg:  "workers count must be positive",
		},
		{
			name: "invalid workers count - exceeds max",
			config: Config{
				Workers:    maxWorkers + 1,
				Output:     "json",
				BufferSize: 4096,
			},
			wantErr: true,
			errMsg:  "workers count cannot exceed system CPU count * 4",
		},
		{
			name: "invalid output format",
			config: Config{
				Workers:    4,
				Output:     "invalid",
				BufferSize: 4096,
			},
			wantErr: true,
			errMsg:  "invalid output format",
		},
		{
			name: "invalid buffer size - negative",
			config: Config{
				Workers:    4,
				Output:     "json",
				BufferSize: -1,
			},
			wantErr: true,
			errMsg:  "buffer size must be positive",
		},
		{
			name: "invalid buffer size - too small",
			config: Config{
				Workers:    4,
				Output:     "json",
				BufferSize: 63,
			},
			wantErr: true,
			errMsg:  "buffer size must be at least 64 bytes",
		},
		{
			name: "invalid max depth",
			config: Config{
				Workers:    4,
				MaxDepth:   -2,
				Output:     "json",
				BufferSize: 4096,
			},
			wantErr: true,
			errMsg:  "max depth must be -1 (unlimited) or positive",
		},
		{
			name: "invalid rate limit",
			config: Config{
				Workers:    4,
				Output:     "json",
				BufferSize: 4096,
				RateLimit:  -1,
			},
			wantErr: true,
			errMsg:  "rate limit must be non-negative",
		},
		{
			name: "output file without path",
			config: Config{
				Workers:    4,
				Output:     "json",
				BufferSize: 4096,
				OutputFile: "",
			},
			wantErr: false, // Default to stdout
		},
		{
			name: "verbosity level validation",
			config: Config{
				Workers:    4,
				Output:     "json",
				BufferSize: 4096,
				Verbose:    4,
			},
			wantErr: false, // Allow any positive verbosity level
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
