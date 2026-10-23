package quality

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
)

// NodeTools lists the tool names that should be looked up in node_modules/.bin first.
var NodeTools = []string{"biome", "eslint", "prettier", "vitest", "jest", "playwright"}

// LookupBin resolves name: root/node_modules/.bin/<name> (on Windows, <name>.cmd and <name>.exe first) for node tools, then lookPath.
func LookupBin(root, name, goos string, lookPath func(string) (string, error)) (string, bool) {
	if goos == "" {
		goos = runtime.GOOS
	}

	if slices.Contains(NodeTools, name) && root != "" {
		binDir := filepath.Join(root, "node_modules", ".bin")
		var candidates []string
		if goos == "windows" {
			candidates = []string{name + ".cmd", name + ".exe", name}
		} else {
			candidates = []string{name}
		}

		for _, cand := range candidates {
			p := filepath.Join(binDir, cand)
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p, true
			}
		}
	}

	if lookPath != nil {
		if p, err := lookPath(name); err == nil && p != "" {
			return p, true
		}
	}

	return "", false
}
