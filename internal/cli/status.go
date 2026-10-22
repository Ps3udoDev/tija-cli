package cli

import (
	"errors"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/render"
	"github.com/spf13/cobra"
)

func newStatusCmd(deps Deps) *cobra.Command {
	var strict bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the branch and working tree status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			o := newOutput(cmd, deps)
			dir, err := workDir()
			if err != nil {
				return err
			}
			rep, err := engine.Status(cmd.Context(), dir, deps.Env)
			if errors.Is(err, engine.ErrNotARepo) {
				return &ExitError{Code: 2, Err: err}
			}
			if err != nil {
				return err
			}
			render.Status(o.printer, rep)
			if code := rep.ExitCode(strict); code != 0 {
				return &ExitError{Code: code}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "Exit 1 when the branch is nonconforming or detached")
	return cmd
}
