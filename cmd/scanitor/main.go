package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/sonemaro/scanitor/cmd/scanitor/app"
	"github.com/sonemaro/scanitor/internal/config"
	"github.com/sonemaro/scanitor/internal/version"
	"github.com/sonemaro/scanitor/pkg/logger"
	"github.com/sonemaro/scanitor/pkg/output"
)

var (
	// Global flags
	cfgFile     string
	verbosity   int
	noProgress  bool
	noColor     bool
	showVersion bool

	// Structure command flags
	workers    int
	maxDepth   int
	ignore     []string
	outputType string
	outputFile string

	// Full command flags
	rateLimit  int
	bufferSize int

	// Global logger instance
	log logger.Logger
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "scanitor [command] [flags] <path>",
	Short: "A directory structure scanner",
	Long: `scanitor v` + version.Version + `
========================================

A tool for generating directory structure with optional content scanning, supporting
concurrent operations and flexible ignore patterns.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize logger
		log = logger.NewLogger(logger.Config{
			Verbosity: verbosity,
			Output:    os.Stderr,
		})

		// Handle version flag
		if showVersion {
			fmt.Println(version.Version)
			os.Exit(0)
		}
	},
}

var structureCmd = &cobra.Command{
	Use:   "structure [flags] <path>",
	Short: "Generate directory structure only",
	Long: `Generates a directory structure without file contents. This is the default
command when no command is specified.`,
	RunE: runStructure,
}

var fullCmd = &cobra.Command{
	Use:   "full [flags] <path>",
	Short: "Generate structure with file contents",
	Long:  `Generates directory structure including the content of all files.`,
	RunE:  runFull,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		if cmd.Flag("full").Value.String() == "true" {
			fmt.Println(version.FullVersion())
		} else {
			fmt.Println(version.Version)
		}
	},
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().CountVarP(&verbosity, "verbose", "v", "verbose output (can be used multiple times)")
	rootCmd.PersistentFlags().BoolVar(&noProgress, "no-progress", false, "disable progress reporting")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	rootCmd.PersistentFlags().BoolVar(&showVersion, "version", false, "print version information")

	// Structure command flags
	structureCmd.Flags().IntVarP(&workers, "workers", "w", runtime.NumCPU(), "number of concurrent workers")
	structureCmd.Flags().IntVarP(&maxDepth, "max-depth", "d", -1, "maximum directory depth")
	structureCmd.Flags().StringSliceVarP(&ignore, "ignore", "i", []string{}, "patterns to ignore")
	structureCmd.Flags().StringVarP(&outputType, "output", "o", "tree", "output format: json|yaml|tree")
	structureCmd.Flags().StringVarP(&outputFile, "output-file", "f", "", "write output to file instead of stdout")

	// Full command flags (inherits structure flags)
	fullCmd.Flags().IntVarP(&rateLimit, "rate-limit", "r", 0, "rate limit for file operations")
	fullCmd.Flags().IntVarP(&bufferSize, "buffer-size", "b", 4096, "buffer size for file reading")
	fullCmd.Flags().AddFlagSet(structureCmd.Flags())

	// Version command flags
	versionCmd.Flags().BoolP("full", "f", false, "show full version information")

	// Add commands
	rootCmd.AddCommand(structureCmd)
	rootCmd.AddCommand(fullCmd)
	rootCmd.AddCommand(versionCmd)

	// Set structure as default command
	rootCmd.SetHelpTemplate(getCustomHelpTemplate())
}

func runStructure(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("requires exactly one path argument")
	}

	log.WithFields(logger.Fields{
		"path":     args[0],
		"workers":  workers,
		"maxDepth": maxDepth,
		"ignore":   ignore,
		"output":   outputType,
		"file":     outputFile,
	}).Info("Starting structure scan")

	application := app.New(&config.Config{
		Workers:    workers,
		MaxDepth:   maxDepth,
		NoProgress: noProgress,
		NoColor:    noColor,
		Verbose:    verbosity,
	})
	defer application.Shutdown()

	return application.Run(args[0], &app.ScanOptions{
		Format:         output.Format(outputType),
		OutputPath:     outputFile,
		IgnorePatterns: ignore,
		IncludeContent: false,
	})
}

func runFull(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("requires exactly one path argument")
	}

	log.WithFields(logger.Fields{
		"path":       args[0],
		"workers":    workers,
		"maxDepth":   maxDepth,
		"ignore":     ignore,
		"output":     outputType,
		"file":       outputFile,
		"rateLimit":  rateLimit,
		"bufferSize": bufferSize,
	}).Info("Starting full scan")

	application := app.New(&config.Config{
		Workers:    workers,
		MaxDepth:   maxDepth,
		NoProgress: noProgress,
		NoColor:    noColor,
		Verbose:    verbosity,
		RateLimit:  rateLimit,
		BufferSize: bufferSize,
	})
	defer application.Shutdown()

	return application.Run(args[0], &app.ScanOptions{
		Format:         output.Format(outputType),
		OutputPath:     outputFile,
		IgnorePatterns: ignore,
		IncludeContent: true,
	})
}

// Helper function to validate output format
func isValidOutputFormat(format output.Format) bool {
	validFormats := map[output.Format]bool{
		output.FormatTree: true,
		output.FormatJSON: true,
		output.FormatYAML: true,
	}
	return validFormats[format]
}

func getCustomHelpTemplate() string {
	return `{{.Long}}

Usage:
  {{.Use}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}

{{if .HasAvailableLocalFlags}}Structure Command (default):
  scanitor [flags] <path>
  scanitor structure [flags] <path>

  Generates a directory structure without file contents. This is the default
  command when no command is specified.

Full Command:
  scanitor full [flags] <path>

  Generates directory structure including the content of all files.

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}

Structure Command Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}

Full Command Flags:
  [All structure command flags plus:]
  -r, --rate-limit int        Rate limit for file operations (ops/sec, default: unlimited)
  -b, --buffer-size int       Buffer size for file reading in bytes (default: 4096)

Version Information:
  scanitor version     Show version number
  scanitor version -f  Show full version information (build date, commit hash, etc.)

Ignore Patterns:
  The --ignore (-i) flag supports various pattern types:

  Directory Patterns:
    -i "node_modules"         Ignore specific directory
    -i "node_modules/"        Trailing slash explicitly indicates directory
    -i "**/node_modules"      Ignore in any subdirectory
    -i "src/test/fixtures"    Ignore specific directory path
    -i ".*/"                  Ignore all hidden directories

  File Patterns:
    -i "*.log"               Ignore by extension
    -i "*.tmp"               Ignore temporary files
    -i "package-lock.json"   Ignore specific files
    -i "test/*.log"          Ignore files in specific directory
    -i "**/*.tmp"            Ignore pattern in any subdirectory

  Multiple patterns can be combined:
    scanitor -i "node_modules" -i ".git" -i "*.log" -i "dist" /path/to/scan

Note: When using environment variables, patterns are comma-separated

Environment Variables:
  SCANITOR_WORKERS          Number of concurrent workers
  SCANITOR_MAX_DEPTH        Maximum directory depth
  SCANITOR_IGNORE           Comma-separated ignore patterns
  SCANITOR_OUTPUT           Output format (json|yaml|tree)
  SCANITOR_OUTPUT_FILE      Output file path
  SCANITOR_RATE_LIMIT       Rate limit (full command only)
  SCANITOR_BUFFER_SIZE      Buffer size (full command only)
  SCANITOR_NO_PROGRESS      Disable progress reporting
  SCANITOR_NO_COLOR         Disable colored output
  SCANITOR_VERBOSE          Verbosity level (number of 'v's)

Examples:
  # Generate directory structure (default)
  scanitor /path/to/scan
  
  # Structure with specific workers and depth
  scanitor -w 4 -d 3 /path/to/scan
  
  # Structure with ignore patterns
  scanitor -i "node_modules" -i ".git" -i "*.log" /path/to/scan
  
  # Structure with complex ignore patterns
  scanitor \
    -i "node_modules" \
    -i ".git" \
    -i "*.log" \
    -i "dist/**" \
    -i "**/test" \
    -i "**/*.tmp" \
    /path/to/scan
  
  # Output as JSON to file
  scanitor -o json -f structure.json /path/to/scan
  
  # Full scan with content
  scanitor full -b 8192 /path/to/scan
  
  # Full scan with rate limiting
  scanitor full -r 100 -b 8192 /path/to/scan

Output Formats:
  tree (default):
    └── root/
        ├── dir1/
        │   └── file1.txt
        └── dir2/
            └── file2.txt

  json:
    {
      "name": "root",
      "type": "directory",
      "children": [
        {
          "name": "dir1",
          "type": "directory",
          "children": [...]
        }
      ]
    }

  yaml:
    name: root
    type: directory
    children:
      - name: dir1
        type: directory
        children: [...]

For more information and updates, visit: https://github.com/sonemaro/scanitor{{end}}
`
}
