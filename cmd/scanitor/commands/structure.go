package commands

import (
	"github.com/sonemaro/scanitor/cmd/scanitor/app"
	"github.com/sonemaro/scanitor/pkg/output"
	"github.com/spf13/cobra"
)

type structureOptions struct {
	*Options
	outputFormat string
	outputFile   string
	maxDepth     int
	workers      int
	ignore       []string
}

func newStructureCommand(opts *Options) *cobra.Command {
	so := &structureOptions{
		Options: opts,
	}

	cmd := &cobra.Command{
		Use:   "structure [flags] <path>",
		Short: "Generate directory structure",
		Long: `Generate a directory structure representation without file contents.
This is the default command when no command is specified.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}
			return runStructure(args[0], so)
		},
	}

	// Add flags specific to structure command
	cmd.Flags().StringVarP(&so.outputFormat, "output", "o", "tree",
		"output format: tree|json|yaml")
	cmd.Flags().StringVarP(&so.outputFile, "file", "f", "",
		"write output to file instead of stdout")
	cmd.Flags().IntVarP(&so.maxDepth, "max-depth", "d", -1,
		"maximum directory depth to scan")
	cmd.Flags().IntVarP(&so.workers, "workers", "w", 0,
		"number of concurrent workers (default: number of CPUs)")
	cmd.Flags().StringSliceVarP(&so.ignore, "ignore", "i", nil,
		"patterns to ignore (can be specified multiple times)")

	return cmd
}

func runStructure(path string, opts *structureOptions) error {
	application := app.New(opts.Config)
	defer application.Shutdown()

	return application.Run(path, &app.ScanOptions{
		Format:         output.Format(opts.outputFormat),
		OutputPath:     opts.outputFile,
		IgnorePatterns: opts.ignore,
		IncludeContent: false,
	})
}
