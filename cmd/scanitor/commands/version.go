package commands

import (
	"fmt"

	"github.com/sonemaro/scanitor/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCommand(opts *Options) *cobra.Command {
	var showFull bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			if showFull {
				fmt.Println(version.FullVersion())
			} else {
				fmt.Println(version.Version)
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&showFull, "full", "f", false,
		"show full version information")

	return cmd
}
