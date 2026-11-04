// Package rules loads optional repository architecture rules.
package rules

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/comments"
	"gopkg.in/yaml.v3"
)

type Check struct {
	ID, Severity, Message string
	Files                 []string
	Forbid                *regexp.Regexp
	Source                string
}
type AgentRule struct {
	ID, Title, Text, Source string
	AppliesTo               []string
}
type File struct {
	Name, Description, Path string
	AppliesTo               []string
	Checks                  []Check
	Rules                   []AgentRule
}
type Set struct{ Files []File }
type stringList []string

func (s *stringList) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode {
		var v string
		if err := n.Decode(&v); err != nil {
			return err
		}
		*s = []string{v}
		return nil
	}
	var v []string
	if err := n.Decode(&v); err != nil {
		return err
	}
	*s = v
	return nil
}
func normalize(s string) string { return strings.ReplaceAll(s, "\\", "/") }
func matches(globs []string, p string) bool {
	if len(globs) == 0 {
		return true
	}
	for _, g := range globs {
		if comments.Match(normalize(g), normalize(p)) {
			return true
		}
	}
	return false
}
func (s Set) Checks() []Check {
	var out []Check
	for _, f := range s.Files {
		out = append(out, f.Checks...)
	}
	return out
}
func (s Set) AgentRules(paths []string) []AgentRule {
	var out []AgentRule
	for _, f := range s.Files {
		applies := len(f.AppliesTo) == 0
		for _, p := range paths {
			applies = applies || matches(f.AppliesTo, p)
		}
		if applies {
			out = append(out, f.Rules...)
		}
	}
	return out
}

// Load validates every matched file and returns errors with source locations.
func Load(root string, globs []string, reserved []string) (Set, []error) {
	var set Set
	var errs []error
	var paths []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, e := filepath.Rel(root, p)
		if e != nil {
			return e
		}
		if len(globs) > 0 && matches(globs, rel) {
			paths = append(paths, normalize(rel))
		}
		return nil
	})
	if err != nil {
		return set, []error{err}
	}
	slices.Sort(paths)
	seen := map[string]string{}
	for _, id := range reserved {
		seen[id] = "reserved pack rule"
	}
	names := map[string]bool{}
	for _, p := range paths {
		data, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		if e != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p, e))
			continue
		}
		f, e := parse(p, string(data))
		if e != nil {
			errs = append(errs, e)
			continue
		}
		if names[f.Name] {
			errs = append(errs, fmt.Errorf("%s: duplicate name %q", p, f.Name))
		}
		names[f.Name] = true
		register := func(id string) {
			if id == "" {
				errs = append(errs, fmt.Errorf("%s: empty rule id", p))
				return
			}
			if prev, ok := seen[id]; ok {
				errs = append(errs, fmt.Errorf("%s: duplicate or reserved id %q (%s)", p, id, prev))
			}
			seen[id] = p
		}
		for _, c := range f.Checks {
			register(c.ID)
		}
		for _, r := range f.Rules {
			register(r.ID)
		}
		set.Files = append(set.Files, f)
	}
	return set, errs
}
func parse(source, text string) (File, error) {
	f := File{Path: source}
	text = strings.TrimPrefix(strings.ReplaceAll(text, "\r\n", "\n"), "\ufeff")
	lines := strings.Split(text, "\n")
	if len(lines) < 2 || lines[0] != "---" {
		return f, fmt.Errorf("%s: missing frontmatter/name", source)
	}
	end := 1
	for end < len(lines) && lines[end] != "---" {
		end++
	}
	if end == len(lines) {
		return f, fmt.Errorf("%s: unclosed frontmatter", source)
	}
	var fm struct {
		Name        string     `yaml:"name"`
		Description string     `yaml:"description"`
		AppliesTo   stringList `yaml:"appliesTo"`
		Checks      []struct {
			ID       string     `yaml:"id"`
			Severity string     `yaml:"severity"`
			Message  string     `yaml:"message"`
			Files    stringList `yaml:"files"`
			Forbid   string     `yaml:"forbid"`
		} `yaml:"checks"`
	}
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &fm); err != nil {
		return f, fmt.Errorf("%s: %w", source, err)
	}
	if strings.TrimSpace(fm.Name) == "" {
		return f, fmt.Errorf("%s: missing name", source)
	}
	f.Name, f.Description, f.AppliesTo = fm.Name, fm.Description, fm.AppliesTo
	for _, c := range fm.Checks {
		severity := ""
		for _, v := range []string{"Critical", "High", "Medium", "Low"} {
			if strings.EqualFold(v, c.Severity) {
				severity = v
			}
		}
		if severity == "" {
			return f, fmt.Errorf("%s: %s: invalid severity %q", source, c.ID, c.Severity)
		}
		if c.Forbid == "" {
			return f, fmt.Errorf("%s: %s: missing forbid", source, c.ID)
		}
		re, err := regexp.Compile(c.Forbid)
		if err != nil {
			return f, fmt.Errorf("%s: %s: %w", source, c.ID, err)
		}
		f.Checks = append(f.Checks, Check{ID: c.ID, Severity: severity, Message: c.Message, Files: c.Files, Forbid: re, Source: source})
	}
	var current *AgentRule
	flush := func() {
		if current != nil {
			current.Text = strings.TrimSpace(current.Text)
			f.Rules = append(f.Rules, *current)
			current = nil
		}
	}
	for _, line := range lines[end+1:] {
		if strings.HasPrefix(line, "## ") {
			flush()
			id, title, ok := strings.Cut(strings.TrimPrefix(line, "## "), ":")
			if ok {
				current = &AgentRule{ID: strings.TrimSpace(id), Title: strings.TrimSpace(title), Source: source, AppliesTo: f.AppliesTo}
			}
			continue
		}
		if current != nil {
			current.Text += line + "\n"
		}
	}
	flush()
	return f, nil
}
