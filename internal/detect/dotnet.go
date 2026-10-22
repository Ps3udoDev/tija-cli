package detect

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	slnProjectRe    = regexp.MustCompile(`Project\("\{[^}]+\}"\)\s*=\s*"[^"]*",\s*"([^"]+\.csproj)"`)
	packageRefRe    = regexp.MustCompile(`(?i)<PackageReference\s+[^>]*Include\s*=\s*"([^"]+)"`)
	targetFrameRe   = regexp.MustCompile(`<TargetFrameworks?>\s*([^<]+?)\s*</TargetFrameworks?>`)
	netVersionRe    = regexp.MustCompile(`(?i)\bnet(\d+)\.\d+`)
	testAttributeRe = regexp.MustCompile(`\[(?:Fact|Theory|Test|TestMethod)\b\s*[\](,]`)
)

// testFrameworks maps a lowercase PackageReference to a framework name.
var testFrameworks = map[string]string{
	"xunit":                "xunit",
	"nunit":                "nunit",
	"mstest.testframework": "mstest",
}

func (d *Detection) detectDotnet() {
	d.Stack = StackDotnetAPI
	d.Language = "C#"
	d.PackageManager = "dotnet"
	if len(d.Solutions) == 1 {
		d.Solution = d.Solutions[0]
	}
	d.applySolution()
}

// applySolution recomputes Projects, Framework, Tools and tests for d.Solution.
func (d *Detection) applySolution() {
	d.Projects, d.Framework, d.TestFramework, d.TestCount = nil, "", "", 0
	tools := []string{"dotnet-format"}
	if d.Solution != "" {
		d.Projects = parseSolution(filepath.Join(d.Root, d.Solution))
		var testDirs []string
		var found []string
		for _, p := range d.Projects {
			full := filepath.Join(d.Root, filepath.FromSlash(p))
			content, err := os.ReadFile(full)
			if err != nil {
				continue
			}
			if fw := testFramework(string(content)); fw != "" {
				found = append(found, fw)
				testDirs = append(testDirs, filepath.Dir(full))
				continue
			}
			if d.Framework == "" {
				if m := targetFrameRe.FindStringSubmatch(string(content)); m != nil {
					if v := netVersionRe.FindStringSubmatch(m[1]); v != nil {
						d.Framework = ".NET " + v[1]
					}
				}
			}
		}
		found = sortedUnique(found)
		tools = append(tools, found...)
		if len(found) > 0 {
			d.TestFramework = found[0]
		}
		for _, dir := range testDirs {
			walkFiles(dir, func(p string) {
				if !strings.HasSuffix(p, ".cs") {
					return
				}
				if b, err := os.ReadFile(p); err == nil {
					d.TestCount += len(testAttributeRe.FindAll(b, -1))
				}
			})
		}
	}
	d.Tools = sortedUnique(tools)
}

// parseSolution returns the csproj paths referenced by a .sln, sorted, with '/'.
// It accepts both CRLF and LF line endings.
func parseSolution(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, m := range slnProjectRe.FindAllStringSubmatch(string(b), -1) {
		out = append(out, strings.ReplaceAll(m[1], `\`, "/"))
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func testFramework(csproj string) string {
	for _, m := range packageRefRe.FindAllStringSubmatch(csproj, -1) {
		if fw, ok := testFrameworks[strings.ToLower(m[1])]; ok {
			return fw
		}
	}
	return ""
}
