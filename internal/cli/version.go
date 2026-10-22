package cli

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

// Version is overridden at build time via ldflags.
var Version = "dev"

// currentVersion is Version, or the module version recorded by
// `go install ...@vX` when no ldflags were set, without a leading "v".
func currentVersion() string { return resolveVersion(Version, debug.ReadBuildInfo) }

func resolveVersion(version string, readBuildInfo func() (*debug.BuildInfo, bool)) string {
	if version != "dev" {
		return strings.TrimPrefix(version, "v")
	}
	if info, ok := readBuildInfo(); ok && info != nil {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return strings.TrimPrefix(v, "v")
		}
	}
	return version
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the Tija version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "tija %s\n", currentVersion())
			return err
		},
	}
}
