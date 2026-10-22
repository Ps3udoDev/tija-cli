package cli

import (
	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/render"
	"github.com/spf13/cobra"
)

func newDoctorCmd(deps Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check the environment and the repository setup",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			o := newOutput(cmd, deps)
			dir, err := workDir()
			if err != nil {
				return err
			}
			rep, err := engine.Doctor(cmd.Context(), dir, deps.Env)
			if err != nil {
				return err
			}
			render.Doctor(o.printer, rep)
			if code := rep.ExitCode(); code != 0 {
				return &ExitError{Code: code}
			}
			return nil
		},
	}
}
