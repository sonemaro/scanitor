/*
Package output provides formatters for scan results in various formats including
tree view, JSON, and YAML. It supports colored output and statistics inclusion.

Basic usage:

	formatter := output.NewFormatter(output.Config{
		Format:     output.FormatTree,
		WithStats:  true,
		WithColors: true,
	}, log)

	result, err := formatter.Format(node)
*/
package output

import (
	"fmt"

	"github.com/sonemaro/scanitor/pkg/logger"
	"github.com/sonemaro/scanitor/pkg/scanner"
)

// Format represents the output format type
type Format string

const (
	FormatTree Format = "tree"
	FormatJSON Format = "json"
	FormatYAML Format = "yaml"
)

// Config holds formatter configuration
type Config struct {
	Format     Format
	WithStats  bool
	WithColors bool
}

// Formatter defines the interface for output formatting
type Formatter interface {
	Format(*scanner.Node) (string, error)
}

// formatter implements the Formatter interface
type formatter struct {
	config Config
	log    logger.Logger
}

// NewFormatter creates a new formatter instance
func NewFormatter(config Config, log logger.Logger) Formatter {
	return &formatter{
		config: config,
		log:    log,
	}
}

// Format formats the node tree according to the configured format
func (f *formatter) Format(node *scanner.Node) (string, error) {
	if node == nil {
		msg := "nil tree provided for formatting"
		f.log.Error(msg) // This matches the test expectation
		return "", fmt.Errorf(msg)
	}

	f.log.WithFields(logger.Fields{
		"format":     f.config.Format,
		"withStats":  f.config.WithStats,
		"withColors": f.config.WithColors,
	}).Debug("Starting format operation")

	switch f.config.Format {
	case FormatTree:
		return f.formatTree(node)
	case FormatJSON:
		return f.formatJSON(node)
	case FormatYAML:
		return f.formatYAML(node)
	default:
		msg := fmt.Sprintf("unsupported format: %s", f.config.Format)
		f.log.Error(msg)
		return "", fmt.Errorf(msg)
	}
}
