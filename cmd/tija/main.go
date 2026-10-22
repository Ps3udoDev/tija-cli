// Command tija is the entry point of the Tija CLI.
package main

import (
	"os"

	"github.com/Ps3udoDev/tija-cli/internal/cli"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"golang.org/x/term"
)

func main() {
	deps := cli.Deps{
		Env: sys.Default(),
		IsTTY: func(v any) bool {
			f, ok := v.(*os.File)
			return ok && term.IsTerminal(int(f.Fd()))
		},
	}
	os.Exit(cli.ExecuteWith(deps, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
