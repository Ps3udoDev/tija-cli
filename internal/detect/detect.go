// Package detect analyses a repository's filesystem to identify its stack.
// It never calls git; callers pass the remote URL in.
package detect

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Stack identifies the kind of project found at the repository root.
type Stack string

const (
	StackDotnetAPI Stack = "dotnet-api"
	StackReactVite Stack = "react-vite"
	StackUnknown   Stack = "unknown"
)

// Detection is the result of analysing a repository root.
type Detection struct {
	Root           string   `json:"root"`
	ProjectName    string   `json:"projectName"`
	Stack          Stack    `json:"stack"`
	Language       string   `json:"language"`
	Framework      string   `json:"framework"`
	Solutions      []string `json:"solutions"`
	Solution       string   `json:"solution"`
	Projects       []string `json:"projects"`
	PackageManager string   `json:"packageManager"`
	Lockfiles      []string `json:"lockfiles"`
	Tools          []string `json:"tools"`
	Scripts        []string `json:"scripts"`
	TestFramework  string   `json:"testFramework"`
	TestCount      int      `json:"testCount"`
	TestFiles      int      `json:"testFiles"` // *.test.* / *.spec.* files, where tests are not counted individually
	CI             []string `json:"ci"`
	GitHost        string   `json:"gitHost"`

	// Filled in by pr.go (Task 5).
	PRTemplate           string   `json:"prTemplate"`
	PRGovernanceWorkflow string   `json:"prGovernanceWorkflow"`
	PRGuide              string   `json:"prGuide"`
	PRTemplateCandidates []string `json:"prTemplateCandidates"`
	BranchPrefixes       []string `json:"branchPrefixes"`
	HuskyBranchHook      bool     `json:"huskyBranchHook"`
	CommitlintHeaderMax  int      `json:"commitlintHeaderMax"`
}

// skipDirs are never descended into when scanning for sources.
var skipDirs = map[string]bool{
	"node_modules": true, "bin": true, "obj": true, "dist": true, ".git": true,
	".next": true, "coverage": true, "build": true, "out": true,
}

// Detect inspects root and reports its stack. remoteURL is only used to
// derive GitHost.
func Detect(root, remoteURL string) (Detection, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Detection{}, err
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		return Detection{}, fmt.Errorf("detect: %s is not a directory", root)
	}
	d := Detection{Root: abs, ProjectName: filepath.Base(abs), Stack: StackUnknown}
	if strings.Contains(remoteURL, "github.com") {
		d.GitHost = "github"
	}

	for _, pattern := range []string{"*.sln", "*.slnx"} {
		slns, err := filepath.Glob(filepath.Join(abs, pattern))
		if err != nil {
			return Detection{}, err
		}
		for _, s := range slns {
			d.Solutions = append(d.Solutions, filepath.Base(s))
		}
	}
	d.Solutions = sortedUnique(d.Solutions)

	switch {
	case len(d.Solutions) > 0 || len(globRel(abs, "*.csproj")) > 0:
		d.detectDotnet()
	case fileExists(filepath.Join(abs, "package.json")):
		d.detectNode()
	}
	d.CI = globRel(abs, ".github/workflows/*.yml", ".github/workflows/*.yaml")
	d.detectPR()
	return d, nil
}

// HasTests reports whether any tests or test files were found.
func (d Detection) HasTests() bool { return d.TestCount > 0 || d.TestFiles > 0 }

// WithSolution returns a copy of d recomputed for the chosen solution.
func (d Detection) WithSolution(sln string) (Detection, error) {
	if !slices.Contains(d.Solutions, sln) {
		return d, fmt.Errorf("detect: solution %q not found in %s", sln, d.Root)
	}
	out := d
	out.Solution = sln
	out.applySolution()
	out.addPRTools()
	return out, nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// globRel returns the sorted, slash-separated relative matches of patterns.
func globRel(root string, patterns ...string) []string {
	var out []string
	for _, p := range patterns {
		m, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(p)))
		for _, f := range m {
			if rel, err := filepath.Rel(root, f); err == nil {
				out = append(out, filepath.ToSlash(rel))
			}
		}
	}
	slices.Sort(out)
	return out
}

// walkFiles calls fn for each file under dir, skipping skipDirs.
func walkFiles(dir string, fn func(path string)) {
	_ = filepath.WalkDir(dir, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if e.IsDir() {
			if p != dir && skipDirs[e.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		fn(p)
		return nil
	})
}

func sortedUnique(in []string) []string {
	slices.Sort(in)
	return slices.Compact(in)
}
