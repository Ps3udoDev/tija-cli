package cli

import (
	"errors"
	"io"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/render"
	"github.com/spf13/cobra"
)

func newInitCmd(deps Deps, in io.Reader) *cobra.Command {
	var presets []string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Set up .tija/ in the current repository",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			o := newOutput(cmd, deps)
			dir, err := workDir()
			if err != nil {
				return err
			}
			plan, err := engine.PlanInit(cmd.Context(), dir, deps.Env)
			if errors.Is(err, engine.ErrNotARepo) {
				return &ExitError{Code: 2, Err: err}
			}
			if err != nil {
				return err
			}

			if err := plan.AddRulesPresets(presets...); err != nil {
				return &ExitError{Code: 2, Err: err}
			}
			var answers map[string]string
			approved := map[string]bool{}
			switch {
			case o.yes:
				for _, a := range plan.Actions {
					if !a.OptIn {
						approved[a.ID] = true
					}
				}
			case !o.json && isTTY(deps, in) && isTTY(deps, cmd.OutOrStdout()):
				answers, approved = ask(newStdinPrompter(in, o.printer), plan)
				o.printer.Line("")
			}
			for _, name := range presets {
				approved["create:.tija/rules/architecture/"+name+".md"] = true
			}
			if len(presets) > 0 {
				approved["update:"+manifest.Path] = true
			}

			res, err := engine.ApplyInit(cmd.Context(), plan, answers, approved)
			if err != nil {
				return err
			}
			render.InitResult(o.printer, res)
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&presets, "rules-preset", nil, "Install an optional architecture rules preset (repeatable)")
	return cmd
}

// ask resolves the plan's choices and the approval of each action that
// needs consent.
func ask(pr prompter, plan engine.InitPlan) (map[string]string, map[string]bool) {
	answers := map[string]string{}
	for _, c := range plan.Choices {
		answers[c.ID] = pr.Choose(c)
	}
	approved := map[string]bool{}
	for _, a := range plan.Actions {
		if a.NeedsConsent {
			approved[a.ID] = pr.Confirm(a)
		}
	}
	return answers, approved
}
