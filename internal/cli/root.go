// Package cli wires the Tija command-line interface.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/spf13/cobra"
)

// ExitError carries a specific process exit code.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit code %d", e.Code)
	}
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error { return e.Err }

// Deps are the injectable dependencies of the CLI.
type Deps struct {
	Env    sys.Env
	IsTTY  func(any) bool
	RunTUI func(dir string) error
}

// Execute runs the CLI with default system dependencies and no TUI.
func Execute(args []string, in io.Reader, out, errOut io.Writer) int {
	return ExecuteWith(Deps{Env: sys.Default()}, args, in, out, errOut)
}

// ExecuteWith runs the CLI with the given dependencies and returns the exit code.
func ExecuteWith(deps Deps, args []string, in io.Reader, out, errOut io.Writer) int {
	root := newRootCmd(deps, in)
	root.SetArgs(args)
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(errOut)

	err := root.Execute()
	if err == nil {
		return 0
	}
	code := 2
	var ee *ExitError
	if errors.As(err, &ee) {
		code = ee.Code
	}
	if msg := err.Error(); msg != "" {
		fmt.Fprintf(errOut, "✗ %s\n", msg)
	}
	return code
}

func newRootCmd(deps Deps, in io.Reader) *cobra.Command {
	root := &cobra.Command{
		Use:           "tija",
		Short:         "Tija, a repo-native engineering orchestrator",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			jsonOut, _ := cmd.Flags().GetBool("json")
			interactive := !jsonOut && deps.RunTUI != nil && deps.IsTTY != nil &&
				deps.IsTTY(in) && deps.IsTTY(cmd.OutOrStdout())
			if !interactive {
				return cmd.Help()
			}
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			return deps.RunTUI(cwd)
		},
	}
	root.PersistentFlags().Bool("json", false, "Output machine-readable JSON")
	root.PersistentFlags().Bool("yes", false, "Approve all actions that require consent")
	root.PersistentFlags().Bool("no-color", false, "Disable colored output")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &ExitError{Code: 2, Err: err}
	})
	root.AddCommand(newVersionCmd())
	return root
}
