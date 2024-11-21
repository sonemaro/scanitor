/*
Package config provides configuration management for the Scanitor application.
It handles both environment variables and validation of all configuration parameters.

Usage:

	cfg, err := config.Load()
	if err != nil {
	    log.Fatal(err)
	}

Environment Variables:

	SCANITOR_WORKERS          Number of concurrent workers
	SCANITOR_MAX_DEPTH        Maximum directory depth
	SCANITOR_IGNORE           Comma-separated ignore patterns
	SCANITOR_OUTPUT           Output format: json|yaml|tree
	SCANITOR_OUTPUT_FILE      Output file path
	SCANITOR_RATE_LIMIT      Rate limit for file operations
	SCANITOR_BUFFER_SIZE     Buffer size for file reading
	SCANITOR_NO_PROGRESS     Disable progress reporting
	SCANITOR_NO_COLOR        Disable colored output
	SCANITOR_VERBOSE         Verbosity level (number of 'v's)

Default Values:

	Workers:     Number of CPU cores
	MaxDepth:    -1 (unlimited)
	Output:      "tree"
	BufferSize:  4096 bytes
	RateLimit:   0 (unlimited)
*/
package config

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration parameters for the application
type Config struct {
	// Workers is the number of concurrent workers for directory scanning
	Workers int

	// MaxDepth is the maximum directory depth to scan (-1 for unlimited)
	MaxDepth int

	// IgnorePatterns is a list of patterns to ignore during scanning
	IgnorePatterns []string

	// Output specifies the output format (tree, json, or yaml)
	Output string

	// OutputFile is the path to write the output (empty for stdout)
	OutputFile string

	// RateLimit is the maximum number of file operations per second (0 for unlimited)
	RateLimit int

	// BufferSize is the size of the buffer for file reading
	BufferSize int

	// NoProgress disables progress reporting
	NoProgress bool

	// NoColor disables colored output
	NoColor bool

	// Verbose sets the verbosity level
	Verbose int
}

// validOutputFormats contains the list of supported output formats
var validOutputFormats = map[string]bool{
	"tree": true,
	"json": true,
	"yaml": true,
}

// Load reads configuration from environment variables and validates it
func Load() (Config, error) {
	v := viper.New()

	// Set default values
	v.SetDefault("workers", runtime.NumCPU())
	v.SetDefault("max_depth", -1)
	v.SetDefault("output", "tree")
	v.SetDefault("buffer_size", 4096)
	v.SetDefault("rate_limit", 0)
	v.SetDefault("no_progress", false)
	v.SetDefault("no_color", false)
	v.SetDefault("verbose", 0)

	// Configure environment variables
	v.SetEnvPrefix("SCANITOR")
	v.AutomaticEnv()

	// Map environment variables to config fields
	v.BindEnv("workers")
	v.BindEnv("max_depth")
	v.BindEnv("ignore")
	v.BindEnv("output")
	v.BindEnv("output_file")
	v.BindEnv("rate_limit")
	v.BindEnv("buffer_size")
	v.BindEnv("no_progress")
	v.BindEnv("no_color")
	v.BindEnv("verbose")

	// Process verbosity level from string of 'v's
	if verboseStr := v.GetString("verbose"); verboseStr != "" {
		v.Set("verbose", strings.Count(verboseStr, "v"))
	}

	// Create config instance
	cfg := Config{
		Workers:    v.GetInt("workers"),
		MaxDepth:   v.GetInt("max_depth"),
		Output:     v.GetString("output"),
		OutputFile: v.GetString("output_file"),
		RateLimit:  v.GetInt("rate_limit"),
		BufferSize: v.GetInt("buffer_size"),
		NoProgress: v.GetBool("no_progress"),
		NoColor:    v.GetBool("no_color"),
		Verbose:    v.GetInt("verbose"),
	}

	// Handle special case for workers=0
	if cfg.Workers == 0 {
		cfg.Workers = runtime.NumCPU()
	}

	// Process ignore patterns
	if ignoreStr := v.GetString("ignore"); ignoreStr != "" {
		patterns := strings.Split(ignoreStr, ",")
		cfg.IgnorePatterns = make([]string, 0, len(patterns))
		for _, p := range patterns {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				cfg.IgnorePatterns = append(cfg.IgnorePatterns, trimmed)
			}
		}
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c Config) Validate() error {
	// Validate workers count
	if c.Workers < 0 {
		return fmt.Errorf("workers count must be positive")
	}
	maxWorkers := runtime.NumCPU() * 4
	if c.Workers > maxWorkers {
		return fmt.Errorf("workers count cannot exceed system CPU count * 4")
	}

	// Validate max depth
	if c.MaxDepth < -1 {
		return fmt.Errorf("max depth must be -1 (unlimited) or positive")
	}

	// Validate output format
	if !validOutputFormats[c.Output] {
		return fmt.Errorf("invalid output format: must be one of [tree json yaml]")
	}

	// Validate buffer size
	if c.BufferSize < 0 {
		return fmt.Errorf("buffer size must be positive")
	}
	if c.BufferSize < MinBufferSize {
		return fmt.Errorf("buffer size must be at least 64 bytes")
	}

	// Validate rate limit
	if c.RateLimit < 0 {
		return fmt.Errorf("rate limit must be non-negative")
	}

	return nil
}

// String returns a string representation of the configuration
func (c Config) String() string {
	return fmt.Sprintf(
		"Config{Workers: %d, MaxDepth: %d, Output: %s, BufferSize: %d, "+
			"RateLimit: %d, NoProgress: %v, NoColor: %v, Verbose: %d, "+
			"IgnorePatterns: %v, OutputFile: %s}",
		c.Workers, c.MaxDepth, c.Output, c.BufferSize,
		c.RateLimit, c.NoProgress, c.NoColor, c.Verbose,
		c.IgnorePatterns, c.OutputFile,
	)
}
