package quality

import (
	"context"
	"errors"
	"net/url"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

// DiagCounts maps slash-relative file path to diagnostic category to count.
type DiagCounts map[string]map[string]int

// Regression records a category whose diagnostic count increased or appeared in after.
type Regression struct {
	Path     string
	Category string
	Before   int
	After    int
}

// BiomeDiagnostics resolves the biome binary using LookupBin(root, "biome", runtime.GOOS, env.LookPath).
// If biome is not found, it returns nil, errors.New("biome binary not found").
// It passes files as paths relative to root (with filepath.ToSlash or rel), running:
// biome check --reporter=github --max-diagnostics=none --colors=off <relative-files...>
// Note: fallback to --max-diagnostics=10000 if --max-diagnostics=none is unsupported in older Biome versions.
// Biome's exit code is ignored (biome exits 1 when diagnostics are present).
// It parses stdout with ParseBiomeGithub(out, root).
func BiomeDiagnostics(ctx context.Context, env sys.Env, root string, files []string) (DiagCounts, error) {
	bin, ok := LookupBin(root, "biome", runtime.GOOS, env.LookPath)
	if !ok {
		return nil, errors.New("biome binary not found")
	}

	if len(files) == 0 {
		return make(DiagCounts), nil
	}

	// Note: fallback to --max-diagnostics=10000 if --max-diagnostics=none is unsupported in older Biome versions.
	args := []string{"check", "--reporter=github", "--max-diagnostics=none", "--colors=off"}
	for _, f := range files {
		rel := f
		if filepath.IsAbs(f) {
			if r, err := filepath.Rel(root, f); err == nil {
				rel = r
			}
		} else if root != "" {
			rootClean := filepath.Clean(root)
			fClean := filepath.Clean(f)
			if r, err := filepath.Rel(rootClean, fClean); err == nil && !strings.HasPrefix(r, "..") {
				rel = r
			}
		}
		relPath := filepath.ToSlash(filepath.Clean(rel))
		relPath = strings.TrimPrefix(relPath, "./")
		args = append(args, relPath)
	}

	stdout, _, _, err := env.Run(ctx, root, bin, args...)
	if err != nil {
		return nil, err
	}

	counts, parsed := ParseBiomeGithub(stdout, root)
	if !parsed {
		return nil, errors.New("unrecognized biome output")
	}
	return counts, nil
}

// ParseBiomeGithub parses GitHub Action format annotations emitted by Biome:
// ::warning title=lint/correctness/noUnusedVariables,file=src\pages\(features)\claims\Form.jsx,line=112,...::message
// or ::error title=...
// Paths are converted to forward-slash relative to root.
// Returns (counts, true). If out is not empty and no lines are recognized, returns (nil, false).
func ParseBiomeGithub(out, root string) (DiagCounts, bool) {
	if strings.TrimSpace(out) == "" {
		return make(DiagCounts), true
	}

	counts := make(DiagCounts)
	recognized := 0

	lines := strings.Split(out, "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || !strings.HasPrefix(line, "::") {
			continue
		}

		rest := line[2:]
		msgIdx := strings.Index(rest, "::")
		if msgIdx == -1 {
			continue
		}

		header := rest[:msgIdx]
		parts := strings.SplitN(header, " ", 2)
		cmd := parts[0]
		if cmd != "warning" && cmd != "error" && cmd != "notice" && cmd != "info" {
			continue
		}

		var filePath, title string
		if len(parts) == 2 {
			paramsStr := parts[1]
			for _, token := range strings.Split(paramsStr, ",") {
				k, v, found := strings.Cut(token, "=")
				if !found {
					continue
				}
				k = strings.TrimSpace(k)
				v = strings.TrimSpace(v)
				switch k {
				case "file":
					filePath = v
				case "title":
					title = v
				}
			}
		}

		if title == "" {
			title = cmd
		}

		recognized++

		if filePath != "" {
			normPath := normalizeDiagPath(filePath, root)
			if counts[normPath] == nil {
				counts[normPath] = make(map[string]int)
			}
			counts[normPath][title]++
		}
	}

	if recognized == 0 {
		return nil, false
	}

	return counts, true
}

func normalizeDiagPath(filePath, root string) string {
	if unescaped, err := url.PathUnescape(filePath); err == nil {
		filePath = unescaped
	}
	if root != "" && filepath.IsAbs(filePath) {
		if rel, err := filepath.Rel(root, filePath); err == nil && !strings.HasPrefix(rel, "..") {
			filePath = rel
		}
	}
	filePath = strings.ReplaceAll(filePath, "\\", "/")
	if root != "" {
		rootSlash := strings.ReplaceAll(root, "\\", "/")
		rootSlash = strings.TrimSuffix(rootSlash, "/")
		if strings.HasPrefix(filePath, rootSlash+"/") {
			filePath = strings.TrimPrefix(filePath, rootSlash+"/")
		}
	}
	clean := path.Clean("/" + strings.TrimPrefix(filePath, "/"))
	return strings.TrimPrefix(clean, "/")
}

// Regressions detects if any category in any file increased in count or appeared for the first time in after.
// Decreases or unchanged counts are not regressions.
func Regressions(before, after DiagCounts) []Regression {
	var result []Regression
	for pathKey, cats := range after {
		for cat, afterCount := range cats {
			beforeCount := 0
			if before != nil && before[pathKey] != nil {
				beforeCount = before[pathKey][cat]
			}
			if afterCount > beforeCount {
				result = append(result, Regression{
					Path:     pathKey,
					Category: cat,
					Before:   beforeCount,
					After:    afterCount,
				})
			}
		}
	}

	slices.SortFunc(result, func(a, b Regression) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return strings.Compare(a.Category, b.Category)
	})

	return result
}
