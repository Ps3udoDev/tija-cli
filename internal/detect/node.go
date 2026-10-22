package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var majorRe = regexp.MustCompile(`\d+`)

type packageJSON struct {
	PackageManager  string            `json:"packageManager"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
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
	if json.Unmarshal(b, &pkg) != nil {
		return
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
	d.Framework = "React " + major(reactV) + " + Vite " + major(viteV)

	d.Language = "JavaScript"
	if fileExists(filepath.Join(d.Root, "tsconfig.json")) || hasTSSources(filepath.Join(d.Root, "src")) {
		d.Language = "TypeScript"
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
			d.TestCount++
		}
	})
}

func major(version string) string {
	return majorRe.FindString(version)
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
