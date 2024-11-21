/*
Package commands implements the CLI command structure for Scanitor.
It provides the root command and all subcommands for directory scanning
operations, with proper flag handling and command coordination.
*/
package commands

import (
	"fmt"
	"runtime"

	"github.com/sonemaro/scanitor/internal/config"
	"github.com/sonemaro/scanitor/pkg/logger"
	"github.com/spf13/cobra"
)

// Options holds command-line options that apply to all commands
type Options struct {
	Config     *config.Config
	ConfigPath string
	Verbose    bool
	NoProgress bool
	NoColor    bool
}

// NewRootCommand creates the root command for the application
func NewRootCommand() *cobra.Command {
	opts := &Options{
		Config: &config.Config{
			Workers: runtime.NumCPU(),
		},
	}

	rootCmd := &cobra.Command{
		Use:   "scanitor [command] [flags] <path>",
		Short: "Directory structure scanner and analyzer",
		Long: `Scanitor is a high-performance directory structure scanner and analyzer
that supports concurrent operations and flexible output formats.

It can generate directory structure representations with optional content
scanning, supporting various ignore patterns and output formats.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initializeCommand(cmd, opts)
		},
		SilenceUsage: true,
	}

	// Add persistent flags that apply to all commands
	rootCmd.PersistentFlags().BoolVarP(&opts.Verbose, "verbose", "v", false,
		"enable verbose output (can be used multiple times)")
	rootCmd.PersistentFlags().BoolVar(&opts.NoProgress, "no-progress", false,
		"disable progress reporting")
	rootCmd.PersistentFlags().BoolVar(&opts.NoColor, "no-color", false,
		"disable colored output")

	// Add commands
	rootCmd.AddCommand(
		newStructureCommand(opts),
		newFullCommand(opts),
		newVersionCommand(opts),
	)

	return rootCmd
}

// initializeCommand performs common initialization for all commands
func initializeCommand(cmd *cobra.Command, opts *Options) error {
	// Count verbose flags (-v, -vv, -vvv)
	verbosity := 0
	for _, arg := range cmd.Flags().Args() {
		if arg == "-v" || arg == "--verbose" {
			verbosity++
		}
	}

	// Initialize logger first
	log := logger.NewLogger(logger.Config{
		Verbosity: verbosity,
	})

	log.WithFields(logger.Fields{
		"verbosity": verbosity,
		"command":   cmd.Name(),
	}).Debug("Initializing command")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.WithFields(logger.Fields{
			"error": err,
		}).Error("Failed to load configuration")
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Override config with command line flags
	cfg.Verbose = verbosity
	cfg.NoProgress = opts.NoProgress
	cfg.NoColor = opts.NoColor

	opts.Config = &cfg

	return nil
}
