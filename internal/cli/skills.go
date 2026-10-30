package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/render"
	"github.com/Ps3udoDev/tija-cli/internal/skills"
)

func newSkillsCmd(deps Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Manage and inspect skill packs",
	}
	cmd.AddCommand(
		newSkillsListCmd(deps),
		newSkillsShowCmd(deps),
	)
	return cmd
}

func newSkillsListCmd(deps Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List skill packs with their origin, author, license and commit",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			o := newOutput(cmd, deps)
			dir, err := workDir()
			if err != nil {
				return err
			}
			root, inRepo, err := gitx.FindRoot(cmd.Context(), deps.Env.Run, dir)
			if err != nil || !inRepo {
				return &ExitError{Code: 2, Err: engine.ErrNotARepo}
			}
			m, err := manifest.Load(root)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return &ExitError{Code: 2, Err: err}
			}
			packs, err := skills.List(root, m)
			if err != nil {
				return &ExitError{Code: 2, Err: err}
			}
			render.Skills(o.printer, packs)
			return nil
		},
	}
}

func newSkillsShowCmd(deps Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show details and content of a skill pack",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o := newOutput(cmd, deps)
			name := args[0]
			dir, err := workDir()
			if err != nil {
				return err
			}
			root, inRepo, err := gitx.FindRoot(cmd.Context(), deps.Env.Run, dir)
			if err != nil || !inRepo {
				return &ExitError{Code: 2, Err: engine.ErrNotARepo}
			}
			m, _ := manifest.Load(root)
			pack, err := skills.Load(root, m, name)
			if err != nil {
				return &ExitError{Code: 2, Err: fmt.Errorf("skill %q not found", name)}
			}
			render.SkillShow(o.printer, pack)
			return nil
		},
	}
}
