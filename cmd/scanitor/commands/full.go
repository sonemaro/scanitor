package commands

import (
	"github.com/sonemaro/scanitor/cmd/scanitor/app"
	"github.com/sonemaro/scanitor/pkg/output"
	"github.com/spf13/cobra"
)

type fullOptions struct {
	*Options
	outputFormat   string
	outputFile     string
	maxDepth       int
	workers        int
	ignore         []string
	rateLimit      int
	bufferSize     int
	followSymlinks bool
}

func newFullCommand(opts *Options) *cobra.Command {
	fo := &fullOptions{
		Options: opts,
	}

	cmd := &cobra.Command{
		Use:   "full [flags] <path>",
		Short: "Generate structure with file contents",
		Long: `Generate a directory structure representation including file contents.
Supports content extraction and more detailed analysis.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}
			return runFull(args[0], fo)
		},
	}

	// Add flags specific to full command
	cmd.Flags().StringVarP(&fo.outputFormat, "output", "o", "tree",
		"output format: tree|json|yaml")
	cmd.Flags().StringVarP(&fo.outputFile, "file", "f", "",
		"write output to file instead of stdout")
	cmd.Flags().IntVarP(&fo.maxDepth, "max-depth", "d", -1,
		"maximum directory depth to scan")
	cmd.Flags().IntVarP(&fo.workers, "workers", "w", 0,
		"number of concurrent workers (default: number of CPUs)")
	cmd.Flags().StringSliceVarP(&fo.ignore, "ignore", "i", nil,
		"patterns to ignore (can be specified multiple times)")
	cmd.Flags().IntVarP(&fo.rateLimit, "rate-limit", "r", 0,
		"rate limit for file operations (ops/sec)")
	cmd.Flags().IntVarP(&fo.bufferSize, "buffer-size", "b", 4096,
		"buffer size for file reading")
	cmd.Flags().BoolVar(&fo.followSymlinks, "follow-symlinks", false,
		"follow symbolic links during scan")

	return cmd
}

func runFull(path string, opts *fullOptions) error {
	application := app.New(opts.Config)
	defer application.Shutdown()

	return application.Run(path, &app.ScanOptions{
		Format:         output.Format(opts.outputFormat),
		OutputPath:     opts.outputFile,
		IgnorePatterns: opts.ignore,
		IncludeContent: true,
		FollowSymlinks: opts.followSymlinks,
	})
}
