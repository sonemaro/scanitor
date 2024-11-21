package config

// OutputFormat represents the supported output formats
type OutputFormat string

const (
	// OutputFormatTree represents the tree-style output format
	OutputFormatTree OutputFormat = "tree"

	// OutputFormatJSON represents the JSON output format
	OutputFormatJSON OutputFormat = "json"

	// OutputFormatYAML represents the YAML output format
	OutputFormatYAML OutputFormat = "yaml"
)

// Constants for configuration limits and defaults
const (
	// MinBufferSize is the minimum allowed buffer size in bytes
	MinBufferSize = 64

	// DefaultBufferSize is the default buffer size in bytes
	DefaultBufferSize = 4096

	// MaxWorkerMultiplier is the maximum multiple of CPU cores for worker count
	MaxWorkerMultiplier = 4

	// UnlimitedDepth represents unlimited directory depth
	UnlimitedDepth = -1
)
