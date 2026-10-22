package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is overridden at build time via ldflags.
var Version = "dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the Tija version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "tija %s\n", Version)
			return err
		},
	}
}
