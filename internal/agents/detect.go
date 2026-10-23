// Package agents detects which AI agent CLIs are installed, invokes them
// non-interactively with the prompt on stdin, and parses their drafts.
package agents

// providers lists the supported providers in preference order with the
// binaries that implement each one, most preferred first.
var providers = []struct {
	name     string
	binaries []string
}{
	{"claude", []string{"claude"}},
	{"codex", []string{"codex"}},
	{"gemini", []string{"agy", "gemini"}}, // agy is Antigravity
}

// Binary returns the resolved path of provider's CLI, or "" when it is not
// installed or the provider is unknown.
func Binary(provider string, lookPath func(string) (string, error)) string {
	for _, p := range providers {
		if p.name != provider {
			continue
		}
		for _, b := range p.binaries {
			if path, err := lookPath(b); err == nil && path != "" {
				return path
			}
		}
	}
	return ""
}

// Available returns the installed providers in order claude, codex, gemini.
func Available(lookPath func(string) (string, error)) []string {
	var out []string
	for _, p := range providers {
		if Binary(p.name, lookPath) != "" {
			out = append(out, p.name)
		}
	}
	return out
}
