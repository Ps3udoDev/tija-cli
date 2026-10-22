package detect

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// majorRe matches the major version of a semver range such as ^19.1.0, ~7,
// >=18 or v7.1.0. Tags (latest), catalog: and workspace: ranges do not match.
var majorRe = regexp.MustCompile(`^\s*(?:[\^~]|[<>]=?|=)?\s*v?(\d+)`)

type packageJSON struct {
	PackageManager  string            `json:"packageManager"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Scripts         map[string]string `json:"scripts"`
}

// lockfiles maps lockfile names to the package manager they imply.
var lockfiles = []struct{ file, manager string }{
	{"bun.lock", "bun"}, {"bun.lockb", "bun"},
	{"pnpm-lock.yaml", "pnpm"}, {"yarn.lock", "yarn"}, {"package-lock.json", "npm"},
}

var toolPackages = map[string]string{
	"@biomejs/biome": "biome", "eslint": "eslint", "prettier": "prettier",
	"@playwright/test": "playwright", "vitest": "vitest", "jest": "jest",
}

func (d *Detection) detectNode() {
	b, err := os.ReadFile(filepath.Join(d.Root, "package.json"))
	if err != nil {
		return
	}
	var pkg packageJSON
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")) // UTF-8 BOM
	if json.Unmarshal(b, &pkg) != nil {
		return
	}
	if len(pkg.Scripts) > 0 {
		var scripts []string
		for s := range pkg.Scripts {
			scripts = append(scripts, s)
		}
		slices.Sort(scripts)
		d.Scripts = scripts
	}

	for _, l := range lockfiles {
		if fileExists(filepath.Join(d.Root, l.file)) {
			d.Lockfiles = append(d.Lockfiles, l.file)
			if d.PackageManager == "" {
				d.PackageManager = l.manager
			}
		}
	}
	d.Lockfiles = sortedUnique(d.Lockfiles)
	// packageManager in package.json wins over lockfiles.
	name, _, _ := strings.Cut(pkg.PackageManager, "@")
	switch name {
	case "bun", "pnpm", "yarn", "npm":
		d.PackageManager = name
	}

	deps := map[string]string{}
	for k, v := range pkg.DevDependencies {
		deps[k] = v
	}
	for k, v := range pkg.Dependencies {
		deps[k] = v
	}
	reactV, hasReact := deps["react"]
	viteV, hasVite := deps["vite"]
	if !hasReact || !hasVite {
		return
	}
	d.Stack = StackReactVite
	d.Framework = withMajor("React", reactV) + " + " + withMajor("Vite", viteV)

	d.Language = "JavaScript"
	if fileExists(filepath.Join(d.Root, "tsconfig.json")) || hasTSSources(filepath.Join(d.Root, "src")) {
		d.Language = "TypeScript"
	}

	var tools []string
	for pkgName, tool := range toolPackages {
		if _, ok := deps[pkgName]; ok {
			tools = append(tools, tool)
		}
	}
	if fileExists(filepath.Join(d.Root, "biome.json")) || fileExists(filepath.Join(d.Root, "biome.jsonc")) {
		tools = append(tools, "biome")
	}
	d.Tools = sortedUnique(tools)

	walkFiles(d.Root, func(p string) {
		base := filepath.Base(p)
		if strings.Contains(base, ".test.") || strings.Contains(base, ".spec.") {
			d.TestFiles++
		}
	})
}

func major(version string) string {
	if m := majorRe.FindStringSubmatch(version); m != nil {
		return m[1]
	}
	return ""
}

// withMajor appends the major version of version to name, or omits it when
// the range has none.
func withMajor(name, version string) string {
	if v := major(version); v != "" {
		return name + " " + v
	}
	return name
}

func hasTSSources(dir string) bool {
	found := false
	walkFiles(dir, func(p string) {
		if strings.HasSuffix(p, ".ts") || strings.HasSuffix(p, ".tsx") {
			found = true
		}
	})
	return found
}
