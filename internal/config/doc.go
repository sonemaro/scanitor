// Package config provides configuration management for the Scanitor application.
// It handles environment variables, command-line flags, and validation of all
// configuration parameters.
//
// # Configuration Loading
//
// To load the configuration:
//
//	cfg, err := config.Load()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// # Environment Variables
//
// The following environment variables are supported:
//
//	SCANITOR_WORKERS      Number of concurrent workers (default: CPU cores)
//	SCANITOR_MAX_DEPTH    Maximum directory depth (-1 for unlimited)
//	SCANITOR_IGNORE       Comma-separated ignore patterns
//	SCANITOR_OUTPUT       Output format: json|yaml|tree
//	SCANITOR_OUTPUT_FILE  Output file path (empty for stdout)
//	SCANITOR_RATE_LIMIT   Rate limit for file operations (0 for unlimited)
//	SCANITOR_BUFFER_SIZE  Buffer size for file reading (default: 4096)
//	SCANITOR_NO_PROGRESS  Disable progress reporting (true/false)
//	SCANITOR_NO_COLOR     Disable colored output (true/false)
//	SCANITOR_VERBOSE      Verbosity level (number of 'v's)
//
// # Example Usage
//
// Basic usage with default configuration:
//
//	cfg, err := config.Load()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Using %d workers\n", cfg.Workers)
//
// Setting environment variables:
//
//	os.Setenv("SCANITOR_WORKERS", "4")
//	os.Setenv("SCANITOR_MAX_DEPTH", "10")
//	os.Setenv("SCANITOR_IGNORE", "node_modules,.git,*.tmp")
//
//	cfg, err := config.Load()
//	// ...
//
// # Configuration Validation
//
// The package performs validation on all configuration values:
//   - Workers must be positive and not exceed CPU cores * 4
//   - MaxDepth must be -1 (unlimited) or positive
//   - Output format must be one of: tree, json, yaml
//   - BufferSize must be at least 64 bytes
//   - RateLimit must be non-negative
//
// # Default Values
//
// The following defaults are applied if not specified:
//   - Workers:     Number of CPU cores
//   - MaxDepth:    -1 (unlimited)
//   - Output:      "tree"
//   - BufferSize:  4096 bytes
//   - RateLimit:   0 (unlimited)
//   - NoProgress:  false
//   - NoColor:     false
//   - Verbose:     0
//
// # Ignore Patterns
//
// The SCANITOR_IGNORE environment variable supports various pattern types:
//
// Directory Patterns:
//   - "node_modules"     - Ignore specific directory
//   - "node_modules/"    - Trailing slash explicitly indicates directory
//   - "**/node_modules"  - Ignore in any subdirectory
//   - "src/test/*"       - Ignore all files in specific directory
//
// File Patterns:
//   - "*.log"           - Ignore by extension
//   - "*.tmp"           - Ignore temporary files
//   - "package.json"    - Ignore specific files
//   - "test/*.log"      - Ignore files in specific directory
//
// Multiple patterns can be combined using commas:
//
//	SCANITOR_IGNORE="node_modules,.git,*.log,dist"
//
// # Output Formats
//
// The package supports three output formats:
//
// Tree Format (default):
//
//	└── root/
//	    ├── dir1/
//	    │   └── file1.txt
//	    └── dir2/
//	        └── file2.txt
//
// JSON Format:
//
//	{
//	  "name": "root",
//	  "type": "directory",
//	  "children": [
//	    {
//	      "name": "dir1",
//	      "type": "directory",
//	      "children": [...]
//	    }
//	  ]
//	}
//
// YAML Format:
//
//	name: root
//	type: directory
//	children:
//	  - name: dir1
//	    type: directory
//	    children: [...]
//
// # Error Handling
//
// The package returns detailed error messages for invalid configurations:
//
//	cfg, err := config.Load()
//	if err != nil {
//	    // Error messages are descriptive:
//	    // "workers count must be positive"
//	    // "invalid output format: must be one of [tree json yaml]"
//	    // "buffer size must be at least 64 bytes"
//	    log.Fatal(err)
//	}
//
// # Thread Safety
//
// The configuration is immutable after loading and is safe for concurrent access
// across multiple goroutines.
//
// # Performance Considerations
//
// The package uses viper internally for configuration management, which provides
// efficient environment variable parsing and type conversion. Configuration
// loading is designed to be performed once at application startup.
//
// # See Also
//
// For more information about the Scanitor application:
// https://github.com/sonemaro/scanitor
//
// Related Packages:
//   - "github.com/sonemaro/scanitor/internal/logger"  - Logging package
//   - "github.com/sonemaro/scanitor/pkg/scanner"      - Directory scanning
//   - "github.com/sonemaro/scanitor/pkg/worker"       - Worker pool implementation
package config
