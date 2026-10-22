// Package branch classifies git branch names against a naming convention.
package branch

import (
	"regexp"
	"strconv"
	"strings"
)

// DefaultPrefixes are the allowed branch type prefixes.
var DefaultPrefixes = []string{"feat", "fix", "refactor", "chore", "docs", "ci", "perf", "test", "build", "style", "revert", "feature", "bugfix", "hotfix", "release"}

// DefaultBases are branch names that are never classified.
var DefaultBases = []string{"main", "master", "dev", "develop"}

// State is the classification outcome.
type State string

const (
	Conforming    State = "conforming"
	Nonconforming State = "nonconforming"
	Base          State = "base"
	Detached      State = "detached"
)

// Rules configures classification.
type Rules struct {
	Prefixes []string
	Bases    []string
	Target   string
}

// Verdict is the result of Classify.
type Verdict struct {
	State       State  `json:"state"`
	Name        string `json:"name"`
	Suggested   string `json:"suggested,omitempty"`
	Reason      string `json:"reason,omitempty"`
	IssueNumber int    `json:"issueNumber,omitempty"`
}

const (
	releaseWord = "release"
	placeholder = "short-description"
	maxDescr    = 3
)

var (
	releaseRe = regexp.MustCompile(`^v?\d+(\.\d+)*([-.][0-9a-z]+)*$`)
	splitRe   = regexp.MustCompile(`[^a-z0-9]+`)
	stopwords = set("and", "in", "the", "of", "a", "to", "for", "on", "with", "de", "la", "el", "en", "y", "con")
	generic   = set("changes", "updates", "update", "test", "fix", "wip", "temp")
)

func set(items ...string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, i := range items {
		m[i] = true
	}
	return m
}

func contains(list []string, s string) bool {
	for _, i := range list {
		if i == s {
			return true
		}
	}
	return false
}

// Classify classifies a branch name under the given rules.
func Classify(name string, detached bool, r Rules) Verdict {
	v := Verdict{Name: name}
	if detached {
		v.State = Detached
		return v
	}
	if contains(r.Bases, name) || (r.Target != "" && name == r.Target) {
		v.State = Base
		return v
	}
	if contains(r.Prefixes, releaseWord) {
		if ver, ok := strings.CutPrefix(name, releaseWord+"/"); ok && releaseRe.MatchString(ver) {
			v.State = Conforming
			return v
		}
	}
	var general []string
	for _, p := range r.Prefixes {
		if p != releaseWord {
			general = append(general, regexp.QuoteMeta(p))
		}
	}
	if len(general) > 0 {
		re := regexp.MustCompile(`^(` + strings.Join(general, "|") + `)/(\d+-)?[a-z0-9]+(-[a-z0-9]+)*$`)
		if m := re.FindStringSubmatch(name); m != nil {
			v.State = Conforming
			if m[2] != "" {
				v.IssueNumber, _ = strconv.Atoi(strings.TrimSuffix(m[2], "-"))
			}
			return v
		}
	}
	v.State = Nonconforming
	suggest(&v, name, r)
	return v
}

func suggest(v *Verdict, name string, r Rules) {
	lower := strings.ToLower(name)
	var tokens []string
	for _, t := range splitRe.Split(lower, -1) {
		if t != "" {
			tokens = append(tokens, t)
		}
	}
	issue := 0
	if len(tokens) > 0 && isDigits(tokens[0]) {
		if n, err := strconv.Atoi(tokens[0]); err == nil {
			issue = n
			tokens = tokens[1:]
		}
	}
	v.IssueNumber = issue

	typ, rest, used := inferType(tokens, r.Prefixes)

	var desc []string
	seen := map[string]bool{}
	for i, t := range tokens {
		if i == used {
			t = rest
		}
		if t == "" || t == typ || stopwords[t] || seen[t] {
			continue
		}
		seen[t] = true
		desc = append(desc, t)
		if len(desc) == maxDescr {
			break
		}
	}

	if len(desc) == 0 || generic[lower] {
		v.Suggested = typ + "/" + placeholder
		v.Reason = "generic branch name"
		return
	}
	s := typ + "/"
	if issue > 0 {
		s += strconv.Itoa(issue) + "-"
	}
	v.Suggested = s + strings.Join(desc, "-")
	v.Reason = "does not follow <prefix>/<short-description>"
}

// inferType returns the inferred type, the remainder of the token that
// yielded it (empty when it matched exactly) and that token's index (-1 if none).
func inferType(tokens, prefixes []string) (typ, rest string, idx int) {
	var allowed []string
	for _, p := range prefixes {
		if p != releaseWord {
			allowed = append(allowed, p)
		}
	}
	for i, t := range tokens {
		best := ""
		for _, p := range allowed {
			if (t == p || (strings.HasPrefix(t, p) && len(t) > len(p))) && len(p) > len(best) {
				best = p
			}
		}
		if best != "" {
			return best, strings.TrimPrefix(t, best), i
		}
		if t == "bug" && contains(allowed, "fix") {
			return "fix", "", i
		}
	}
	if contains(allowed, "feat") {
		return "feat", "", -1
	}
	if len(allowed) > 0 {
		return allowed[0], "", -1
	}
	return "feat", "", -1
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}
