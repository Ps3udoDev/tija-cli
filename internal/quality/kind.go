package quality

import (
	"strings"
)

type Kind string

const (
	Build  Kind = "build"
	Lint   Kind = "lint"
	Format Kind = "format"
	Tests  Kind = "tests"
)

var Order = []Kind{Build, Lint, Format, Tests}

type Mode int

const (
	CheckMode Mode = iota
	FixMode
)

// Invocation is one process to run; it may cover several kinds (biome ci = lint + format).
type Invocation struct {
	Kinds    []Kind
	Provider string
	Bin      string   // logical name ("biome", "dotnet", "bun", "npm")
	Path     string   // resolved executable path; "" if unavailable
	Args     []string
	Report   string   // absolute path of test report to read afterwards ("" if none)
	Hint     string   // installation hint if Path == ""
}

// Display returns a human-readable command string: Bin + Args, quoting args that contain spaces.
func (i Invocation) Display() string {
	parts := []string{i.Bin}
	for _, arg := range i.Args {
		if strings.Contains(arg, " ") {
			parts = append(parts, `"`+arg+`"`)
		} else {
			parts = append(parts, arg)
		}
	}
	return strings.Join(parts, " ")
}
