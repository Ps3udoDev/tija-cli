package render

import (
	"fmt"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/detect"
	"github.com/Ps3udoDev/tija-cli/internal/engine"
)

// none is shown for a field with no value.
const none = "-"

// InitResult renders the summary of tija init: what was detected, created,
// updated and what still waits for consent.
func InitResult(p Printer, r engine.InitResult) {
	if p.Mode == JSON {
		_ = p.JSONOut(initJSON(r))
		return
	}
	p.Heading("Detected project")
	p.Line("")
	detection(p, r.Detection)

	block := func(title string, paths []string) {
		if len(paths) == 0 {
			return
		}
		p.Line("")
		p.Heading(title)
		for _, path := range paths {
			p.Ok(path)
		}
	}
	block("Created:", r.Created)
	block("Updated:", r.Updated)

	if len(r.Pending) > 0 {
		p.Line("")
		p.Heading("Pending (run with --yes or interactively):")
		for _, a := range r.Pending {
			msg := string(a.Kind) + " " + a.Path
			if a.Reason != "" {
				msg += " — " + a.Reason
			}
			p.Warn(msg)
		}
	}
	if len(r.Created)+len(r.Updated)+len(r.Pending) == 0 {
		p.Line("")
		p.Ok("already up to date")
	}
	if len(r.Notes) > 0 {
		p.Line("")
		for _, n := range r.Notes {
			p.Warn(n)
		}
	}
}

func detection(p Printer, d detect.Detection) {
	p.Field("Language", orNone(d.Language))
	p.Field("Framework", orNone(d.Framework))
	if d.Solution != "" {
		p.Field("Solution", d.Solution)
	}
	p.Field("Packages", orNone(d.PackageManager))
	p.Field("Tests", tests(d))
	p.Field("Tools", orNone(strings.Join(d.Tools, ", ")))
	git := none
	if d.GitHost == "github" {
		git = "GitHub"
	}
	p.Field("Git", git)
	ci := none
	if n := len(d.CI); n > 0 {
		ci = fmt.Sprintf("GitHub Actions (%d %s)", n, plural(n, "workflow"))
	}
	p.Field("CI", ci)
}

func tests(d detect.Detection) string {
	switch {
	case d.TestCount == 0 && d.TestFramework == "":
		return "none detected"
	case d.TestCount == 0:
		return d.TestFramework
	case d.TestFramework == "":
		return fmt.Sprintf("%d %s", d.TestCount, plural(d.TestCount, "test"))
	}
	return fmt.Sprintf("%s (%d %s)", d.TestFramework, d.TestCount, plural(d.TestCount, "test"))
}

// initJSON returns r with nil lists replaced by empty ones, so consumers
// always get arrays.
func initJSON(r engine.InitResult) engine.InitResult {
	if r.Created == nil {
		r.Created = []string{}
	}
	if r.Updated == nil {
		r.Updated = []string{}
	}
	if r.Pending == nil {
		r.Pending = []engine.Action{}
	}
	if r.Notes == nil {
		r.Notes = []string{}
	}
	return r
}

func orNone(s string) string {
	if s == "" {
		return none
	}
	return s
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
