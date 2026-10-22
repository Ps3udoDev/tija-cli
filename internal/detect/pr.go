package detect

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	branchRegexLine = regexp.MustCompile(`VALID_BRANCH_REGEX=.*?\^\(([^)]*)\)`)
	headerMaxRe     = regexp.MustCompile(`"header-max-length"\s*:\s*\[\s*\d+\s*,\s*"always"\s*,\s*(\d+)`)
	baseBranches    = map[string]bool{"main": true, "master": true, "dev": true, "develop": true}
)

// maxGuideDepth is the maximum number of parent directories of a PR guide.
const maxGuideDepth = 2

// detectPR fills the PR-related fields and the husky/commitlint tools.
// It applies to every stack.
func (d *Detection) detectPR() {
	d.detectPRTemplate()
	d.detectGovernance()
	d.detectGuide()
	d.detectHusky()
	d.detectCommitlint()
	d.addPRTools()
}

// findFold returns the entries of dir (relative to root, slash-separated)
// whose lower-cased name satisfies match, with their real on-disk casing.
func findFold(root, dir string, match func(lower string) bool) []string {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !match(strings.ToLower(e.Name())) {
			continue
		}
		if dir == "" {
			out = append(out, e.Name())
		} else {
			out = append(out, dir+"/"+e.Name())
		}
	}
	slices.Sort(out)
	return out
}

// subDirFold returns the real name of the subdirectory of dir matching name
// case-insensitively, or "".
func subDirFold(root, dir, name string) string {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() && strings.EqualFold(e.Name(), name) {
			return e.Name()
		}
	}
	return ""
}

func (d *Detection) detectPRTemplate() {
	isTpl := func(l string) bool { return l == "pull_request_template.md" }
	var groups [][]string
	groups = append(groups, findFold(d.Root, ".github", isTpl))
	groups = append(groups, findFold(d.Root, "", isTpl))
	groups = append(groups, findFold(d.Root, "docs", isTpl))
	if gh := subDirFold(d.Root, "", ".github"); gh != "" {
		if sub := subDirFold(d.Root, gh, "pull_request_template"); sub != "" {
			groups = append(groups, findFold(d.Root, gh+"/"+sub, func(l string) bool { return strings.HasSuffix(l, ".md") }))
		}
	}
	var all []string
	for _, g := range groups {
		if d.PRTemplate == "" && len(g) > 0 {
			d.PRTemplate = g[0]
		}
		all = append(all, g...)
	}
	d.PRTemplateCandidates = sortedUnique(all)
}

func (d *Detection) detectGovernance() {
	for _, f := range globRel(d.Root, ".github/workflows/*") {
		b, err := os.ReadFile(filepath.Join(d.Root, filepath.FromSlash(f)))
		if err == nil && strings.Contains(string(b), "action-semantic-pull-request") {
			d.PRGovernanceWorkflow = f
			return
		}
	}
}

func (d *Detection) detectGuide() {
	var found []string
	walkFiles(d.Root, func(p string) {
		rel, err := filepath.Rel(d.Root, p)
		if err != nil {
			return
		}
		rel = filepath.ToSlash(rel)
		parts := strings.Split(rel, "/")
		if parts[0] == ".tija" || len(parts)-1 > maxGuideDepth {
			return
		}
		if strings.EqualFold(parts[len(parts)-1], "PULL_REQUEST_TEMPLATE_GUIDE.md") {
			found = append(found, rel)
		}
	})
	if len(found) > 0 {
		slices.Sort(found)
		d.PRGuide = found[0]
	}
}

func (d *Detection) detectHusky() {
	b, err := os.ReadFile(filepath.Join(d.Root, ".husky", "pre-push"))
	if err != nil || !strings.Contains(string(b), "VALID_BRANCH_REGEX=") {
		return
	}
	d.HuskyBranchHook = true
	m := branchRegexLine.FindStringSubmatch(string(b))
	if m == nil {
		return
	}
	for _, alt := range strings.Split(m[1], "|") {
		if alt = strings.TrimSpace(alt); alt != "" && !baseBranches[alt] {
			d.BranchPrefixes = append(d.BranchPrefixes, alt)
		}
	}
}

func (d *Detection) commitlintFile() string {
	entries, err := os.ReadDir(d.Root)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(n, ".commitlintrc") {
			return n
		}
		for _, ext := range []string{"js", "cjs", "mjs", "ts"} {
			if n == "commitlint.config."+ext {
				return n
			}
		}
	}
	return ""
}

func (d *Detection) detectCommitlint() {
	f := d.commitlintFile()
	if f == "" {
		return
	}
	b, err := os.ReadFile(filepath.Join(d.Root, f))
	if err != nil {
		return
	}
	if m := headerMaxRe.FindSubmatch(b); m != nil {
		d.CommitlintHeaderMax, _ = strconv.Atoi(string(m[1]))
	}
}

// addPRTools adds husky and commitlint to Tools when present. It is safe to
// call again after Tools has been recomputed.
func (d *Detection) addPRTools() {
	tools := slices.Clone(d.Tools)
	if st, err := os.Stat(filepath.Join(d.Root, ".husky")); err == nil && st.IsDir() {
		tools = append(tools, "husky")
	}
	if d.commitlintFile() != "" {
		tools = append(tools, "commitlint")
	}
	d.Tools = sortedUnique(tools)
}
