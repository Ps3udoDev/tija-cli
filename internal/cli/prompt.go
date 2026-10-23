package cli

import (
	"bufio"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/render"
)

// prompter asks the user to approve actions and answer choices.
type prompter interface {
	Confirm(a engine.Action) bool
	Choose(c engine.Choice) string
}

// stdinPrompter reads answers line by line; end of input means "no" for
// confirmations and the default for choices.
type stdinPrompter struct {
	in *bufio.Reader
	p  render.Printer
}

func newStdinPrompter(in io.Reader, p render.Printer) *stdinPrompter {
	return &stdinPrompter{in: bufio.NewReader(in), p: p}
}

// readLine returns the next trimmed line; ok is false at end of input.
func (s *stdinPrompter) readLine() (string, bool) {
	line, err := s.in.ReadString('\n')
	if err != nil && line == "" {
		return "", false
	}
	return strings.TrimSpace(line), true
}

func (s *stdinPrompter) ask(question string) {
	fmt.Fprint(s.p.W, question)
}

// Confirm shows the action and its diff and asks [y/N].
func (s *stdinPrompter) Confirm(a engine.Action) bool {
	s.p.Line("")
	msg := string(a.Kind) + " " + a.Path
	if a.Reason != "" {
		msg += " — " + a.Reason
	}
	s.p.Heading(msg)
	if a.Diff != "" {
		s.p.Line(strings.TrimRight(a.Diff, "\n"))
	}
	s.ask("Apply? [y/N] ")
	line, ok := s.readLine()
	if !ok {
		s.p.Line("")
		return false
	}
	switch strings.ToLower(line) {
	case "y", "yes":
		return true
	}
	return false
}

// Choose lists the options and reads a number or an option name; an empty
// answer or end of input picks the default.
func (s *stdinPrompter) Choose(c engine.Choice) string {
	s.p.Line("")
	s.p.Heading(c.Question)
	for i, o := range c.Options {
		mark := " "
		if o == c.Default {
			mark = ">"
		}
		s.p.Line(fmt.Sprintf("%s %d) %s", mark, i+1, o))
	}
	for {
		s.ask(fmt.Sprintf("Choice [%s]: ", c.Default))
		line, ok := s.readLine()
		if !ok {
			s.p.Line("")
			return c.Default
		}
		if line == "" {
			return c.Default
		}
		if n, err := strconv.Atoi(line); err == nil && n >= 1 && n <= len(c.Options) {
			return c.Options[n-1]
		}
		if slices.Contains(c.Options, line) {
			return line
		}
		s.p.Warn("unknown option: " + line)
	}
}

// Yes asks a [y/N] question; end of input means no.
func (s *stdinPrompter) Yes(question string) bool {
	s.ask(question + " [y/N] ")
	line, ok := s.readLine()
	if !ok {
		s.p.Line("")
		return false
	}
	switch strings.ToLower(line) {
	case "y", "yes":
		return true
	}
	return false
}
