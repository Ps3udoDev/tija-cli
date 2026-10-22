// Package render formats CLI output as styled text, plain text or JSON.
package render

import (
	"encoding/json"
	"fmt"
	"io"
)

// Mode selects how output is rendered.
type Mode int

const (
	Styled Mode = iota
	Plain
	JSON
)

const (
	symOk   = "✓"
	symFail = "✗"
	symWarn = "!"
)

// ModeFor picks the output mode. JSON has priority; Styled requires a TTY
// and no --no-color flag or NO_COLOR variable.
func ModeFor(isTTY bool, jsonFlag, noColor bool, getenv func(string) string) Mode {
	if jsonFlag {
		return JSON
	}
	if isTTY && !noColor && getenv("NO_COLOR") == "" {
		return Styled
	}
	return Plain
}

// Printer writes status lines in the configured Mode.
type Printer struct {
	W    io.Writer
	Mode Mode
}

func (p Printer) Ok(msg string)   { p.line(symOk, okStyle.Render(symOk), msg) }
func (p Printer) Warn(msg string) { p.line(symWarn, warnStyle.Render(symWarn), msg) }
func (p Printer) Fail(msg string) { p.line(symFail, failStyle.Render(symFail), msg) }

func (p Printer) Heading(msg string) {
	if p.Mode == Styled {
		msg = headingStyle.Render(msg)
	}
	fmt.Fprintln(p.W, msg)
}

func (p Printer) KV(key, value string) {
	fmt.Fprintf(p.W, "%s: %s\n", key, value)
}

// JSONOut writes v as indented JSON followed by a newline.
func (p Printer) JSONOut(v any) error {
	enc := json.NewEncoder(p.W)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func (p Printer) line(plain, styled, msg string) {
	sym := plain
	if p.Mode == Styled {
		sym = styled
	}
	fmt.Fprintf(p.W, "%s %s\n", sym, msg)
}
