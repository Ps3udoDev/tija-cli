package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Ps3udoDev/tija-cli/internal/agents"
	"github.com/Ps3udoDev/tija-cli/internal/branch"
	"github.com/Ps3udoDev/tija-cli/internal/prrules"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
)

// Draft files, relative to the repo root.
const (
	DraftPath     = ".tija/session/pr.md"
	DraftMetaPath = ".tija/session/pr.json"
)

// DraftMeta describes how and for which commit a draft was produced.
type DraftMeta struct {
	Head     string            `json:"head"`
	Branch   string            `json:"branch"`
	Target   string            `json:"target"`
	Source   string            `json:"source"`
	Problems []prrules.Problem `json:"problems"`
}

// SaveDraft writes the draft to .tija/session/pr.md as "<title>\n\n<body>"
// with the body normalised to LF, and meta to .tija/session/pr.json.
func SaveDraft(root string, d agents.Draft, meta DraftMeta) error {
	md := abs(root, DraftPath)
	if err := os.MkdirAll(filepath.Dir(md), 0o755); err != nil {
		return err
	}
	title := strings.TrimSpace(strings.ReplaceAll(d.Title, "\r\n", "\n"))
	if err := os.WriteFile(md, []byte(title+"\n\n"+prrules.Normalize(d.Body)), 0o644); err != nil {
		return err
	}
	if meta.Problems == nil {
		meta.Problems = []prrules.Problem{}
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(abs(root, DraftMetaPath), append(data, '\n'), 0o644)
}

// LoadDraft reads the draft saved by SaveDraft. A missing pr.md yields an
// error satisfying errors.Is(err, fs.ErrNotExist); a missing pr.json yields
// an empty DraftMeta.
func LoadDraft(root string) (agents.Draft, DraftMeta, error) {
	data, err := os.ReadFile(abs(root, DraftPath))
	if err != nil {
		return agents.Draft{}, DraftMeta{}, err
	}
	// Editors on Windows may save with a UTF-8 BOM; it is not part of the title.
	s := strings.TrimPrefix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\uFEFF")
	title, body, _ := strings.Cut(s, "\n")
	d := agents.Draft{Title: strings.TrimSpace(title)}
	if strings.TrimSpace(body) != "" {
		d.Body = prrules.Normalize(body)
	}
	var meta DraftMeta
	js, err := os.ReadFile(abs(root, DraftMetaPath))
	switch {
	case err == nil:
		if err := json.Unmarshal(js, &meta); err != nil {
			return d, DraftMeta{}, fmt.Errorf("%s: %w", DraftMetaPath, err)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return d, DraftMeta{}, err
	}
	return d, meta, nil
}

var (
	issuePrefixRx = regexp.MustCompile(`^\d+-`)
	commitRx      = regexp.MustCompile(`^(\w+)(?:\([^)]*\))?!?: (.+)$`)
	headingRx     = regexp.MustCompile(`^##\s+(.+?)\s*$`)
	htmlCommentRx = regexp.MustCompile(`<!--[\s\S]*?-->`)
	typeAlias     = map[string]string{"feature": "feat", "bugfix": "fix", "hotfix": "fix", "release": "chore"}
)

const placeholderDescr = "short-description"

// DeterministicDraft builds a draft without any agent, always in Spanish:
// type and scope from the branch, changes from the commits and tests only
// from the verified results in c.Tests.
func DeterministicDraft(c PRContext) agents.Draft {
	name := c.Branch
	if c.Verdict.State != branch.Conforming && c.Verdict.Suggested != "" {
		name = c.Verdict.Suggested
	}
	prefix, rest, _ := strings.Cut(name, "/")
	typ := strings.ToLower(prefix)
	if a, ok := typeAlias[typ]; ok {
		typ = a
	}
	if !slices.Contains(prrules.Types, typ) {
		typ = "chore"
	}
	descSlug := issuePrefixRx.ReplaceAllString(rest, "")
	if descSlug == placeholderDescr {
		descSlug = ""
	}

	scope, desc := "", ""
	if descSlug != "" {
		scope = strings.Split(descSlug, "-")[0]
		desc = strings.ReplaceAll(descSlug, "-", " ")
	} else {
		scope = topDirScope(c)
		if len(c.Commits) > 0 {
			desc = commitText(c.Commits[len(c.Commits)-1].Subject)
		}
		if desc == "" {
			desc = "cambios de la rama " + c.Branch
		}
	}
	desc = strings.TrimRight(strings.TrimSpace(desc), ".")
	head := typ + "(" + scope + "): "
	title := head + fitWords(desc, c.Manifest.PullRequest.Title.MaxLength-utf8.RuneCountInString(head))

	return agents.Draft{Title: title, Body: deterministicBody(c, desc)}
}

// fitWords shortens s at a word boundary to at most max runes (max <= 0
// means no limit). A single word longer than max is cut.
func fitWords(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	out := ""
	for _, w := range strings.Fields(s) {
		next := w
		if out != "" {
			next = out + " " + w
		}
		if utf8.RuneCountInString(next) > max {
			break
		}
		out = next
	}
	if out == "" {
		out = string([]rune(s)[:max])
	}
	return strings.TrimRight(out, " .")
}

// topDirScope returns the most touched top-level directory (by files, then
// lines), lowercased with '.' and '_' replaced by '-'.
func topDirScope(c PRContext) string {
	type stat struct{ files, lines int }
	dirs := map[string]*stat{}
	for _, f := range c.Files {
		dir, _, ok := strings.Cut(f.Path, "/")
		if !ok {
			continue
		}
		s := dirs[dir]
		if s == nil {
			s = &stat{}
			dirs[dir] = s
		}
		s.files++
		s.lines += f.Added + f.Deleted
	}
	best := ""
	for d, s := range dirs {
		b := dirs[best]
		if best == "" || s.files > b.files || (s.files == b.files && (s.lines > b.lines || (s.lines == b.lines && d < best))) {
			best = d
		}
	}
	if best == "" {
		best = c.Manifest.Project.Name
	}
	best = strings.NewReplacer(".", "-", "_", "-").Replace(strings.ToLower(strings.TrimSpace(best)))
	best = strings.Trim(best, "-")
	if best == "" {
		return "repo"
	}
	return best
}

// commitText strips the "type(scope): " prefix of a conventional subject.
func commitText(subject string) string {
	if m := commitRx.FindStringSubmatch(subject); m != nil {
		return m[2]
	}
	return subject
}

func capitalize(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	if n == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[n:]
}

func deterministicBody(c PRContext, desc string) string {
	pr := c.Manifest.PullRequest
	refs := fmt.Sprintf("Rama `%s`.", c.Branch)
	if c.IssueNumber > 0 {
		refs = fmt.Sprintf("Closes #%d", c.IssueNumber)
	}
	objective := capitalize(desc) + "."
	if c.IssueNumber > 0 {
		objective += fmt.Sprintf(" Relacionado con el issue #%d.", c.IssueNumber)
	}
	if utf8.RuneCountInString(objective) < pr.MinSectionLength {
		objective += fmt.Sprintf(" Cambios de la rama `%s`.", c.Branch)
	}
	fill := map[string]string{
		"resumen": fmt.Sprintf("Cambios de la rama `%s` hacia `%s`: %d commit(s) y %d archivo(s) modificados.",
			c.Branch, c.Target, len(c.Commits), len(c.Files)),
		"objetivo":           objective,
		"cambios realizados": changesList(c),
		"pruebas":            testsList(c.Tests),
		"evidencias":         "N/A",
		"referencias":        refs,
	}

	tpl := htmlCommentRx.ReplaceAllString(strings.ReplaceAll(c.Template, "\r\n", "\n"), "")
	var preamble []string
	type sec struct {
		heading string
		lines   []string
	}
	var secs []sec
	for _, l := range strings.Split(tpl, "\n") {
		if m := headingRx.FindStringSubmatch(l); m != nil {
			secs = append(secs, sec{heading: m[1]})
			continue
		}
		if len(secs) == 0 {
			preamble = append(preamble, l)
		} else {
			secs[len(secs)-1].lines = append(secs[len(secs)-1].lines, l)
		}
	}
	present := map[string]bool{}
	for _, s := range secs {
		present[strings.ToLower(s.heading)] = true
	}
	// Required sections the template lacks are appended when Tija can fill them.
	for _, name := range pr.RequiredSections {
		key := strings.ToLower(name)
		if _, ok := fill[key]; ok && !present[key] {
			secs = append(secs, sec{heading: name})
			present[key] = true
		}
	}

	var parts []string
	if p := strings.TrimSpace(strings.Join(preamble, "\n")); p != "" {
		parts = append(parts, p)
	}
	for _, s := range secs {
		content, ok := fill[strings.ToLower(s.heading)]
		if !ok {
			content = strings.TrimSpace(strings.Join(s.lines, "\n"))
		}
		block := "## " + s.heading
		if content != "" {
			block += "\n\n" + content
		}
		parts = append(parts, block)
	}
	return prrules.Normalize(strings.Join(parts, "\n\n"))
}

// changesList renders one bullet per commit, grouped by conventional type
// in prrules.Types order; non-conventional subjects go last.
func changesList(c PRContext) string {
	groups := map[string][]string{}
	for _, cm := range c.Commits {
		typ, text := "", cm.Subject
		if m := commitRx.FindStringSubmatch(cm.Subject); m != nil {
			if t := strings.ToLower(m[1]); slices.Contains(prrules.Types, t) {
				typ, text = t, m[2]
			}
		}
		groups[typ] = append(groups[typ], "- "+text)
	}
	order := slices.Clone(prrules.Types)
	order = append(order, "")
	var keys []string
	for _, k := range order {
		if len(groups[k]) > 0 {
			keys = append(keys, k)
		}
	}

	var out []string
	for _, k := range keys {
		if len(keys) > 1 {
			label := k
			if label == "" {
				label = "otros"
			}
			if len(out) > 0 {
				out = append(out, "")
			}
			out = append(out, "**"+label+"**", "")
		}
		out = append(out, groups[k]...)
	}
	return strings.Join(out, "\n")
}

// pruebasSection is the section that cites the verified checks.
const pruebasSection = "Pruebas"

// enforceVerifiedTests makes the ## Pruebas section of an agent body cite
// what Tija verified, whatever the agent wrote: in Spanish (the language of
// testsList) the section becomes the verified list; in any other language
// the agent's text stays and the verified list follows it. A body without
// the section is returned as is, so validation reports it.
func enforceVerifiedTests(body, language string, tests []quality.StepResult) string {
	verified := testsList(tests)
	if language == "" || strings.EqualFold(language, "es") {
		return replaceSection(body, pruebasSection, func(string) string { return verified })
	}
	return replaceSection(body, pruebasSection, func(content string) string {
		if content == "" {
			return verified
		}
		return content + "\n\n" + verified
	})
}

// replaceSection replaces the content of the "## <title>" section of a
// markdown body (heading matched case-insensitively, kept as written) with
// edit(content), where content is the current text trimmed. A body without
// the section is returned unchanged.
func replaceSection(body, title string, edit func(content string) string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	head := regexp.MustCompile(`(?im)^##[ \t]+` + regexp.QuoteMeta(title) + `[ \t]*$`)
	loc := head.FindStringIndex(body)
	if loc == nil {
		return body
	}
	rest := body[loc[1]:]
	end := len(rest)
	if n := regexp.MustCompile(`(?m)^##[ \t]+`).FindStringIndex(rest); n != nil {
		end = n[0]
	}
	content := strings.TrimSpace(rest[:end])
	after := rest[end:]
	section := body[:loc[1]]
	if c := strings.TrimSpace(edit(content)); c != "" {
		section += "\n\n" + c
	}
	if after == "" {
		return section + "\n"
	}
	return section + "\n\n" + after
}

// testsList cites only verified results that actually ran.
func testsList(results []quality.StepResult) string {
	var out []string
	for _, r := range results {
		name := r.Command
		if name == "" {
			name = r.Provider
		}
		// Only runs that passed or failed are cited; the status always wins
		// over the counts, so a failed run never reads as passing.
		if r.Status != quality.StatusPassed && r.Status != quality.StatusFailed {
			continue
		}
		var parts []string
		if r.Tests != nil {
			counts := fmt.Sprintf("%d superadas, %d fallidas", r.Tests.Passed, r.Tests.Failed)
			if r.Tests.Skipped > 0 {
				counts += fmt.Sprintf(", %d omitidas", r.Tests.Skipped)
			}
			parts = append(parts, counts)
		}
		switch {
		case r.Status == quality.StatusFailed && r.TimedOut:
			parts = append(parts, "falló por tiempo agotado")
		case r.Status == quality.StatusFailed:
			parts = append(parts, fmt.Sprintf("falló (código de salida %d)", r.ExitCode))
		case r.Tests == nil:
			parts = append(parts, "sin errores")
		}
		out = append(out, fmt.Sprintf("- `%s`: %s.", name, strings.Join(parts, "; ")))
	}
	if len(out) == 0 {
		return "- No se ejecutaron verificaciones automáticas para este commit."
	}
	return strings.Join(out, "\n")
}
