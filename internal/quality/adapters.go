package quality

import (
	"path/filepath"
	"slices"

	"github.com/Ps3udoDev/tija-cli/internal/manifest"
)

type Project struct {
	Root           string
	PackageManager string
	Solution       string
	Scripts        []string
	TmpDir         string
	GOOS           string
}

type Adapter interface {
	Provider() string
	// Plan returns the invocation for kinds in mode, or ok=false when the provider cannot do it.
	Plan(p Project, e manifest.QualityEntry, kinds []Kind, mode Mode) (Invocation, bool)
}

func nodePM(p Project) string {
	if p.PackageManager != "" {
		return p.PackageManager
	}
	return "npm"
}

// BiomeAdapter handles biome. Supports grouping lint + format.
type biomeAdapter struct{}

func (biomeAdapter) Provider() string { return "biome" }
func (biomeAdapter) Plan(p Project, e manifest.QualityEntry, kinds []Kind, mode Mode) (Invocation, bool) {
	hasLint := slices.Contains(kinds, Lint)
	hasFormat := slices.Contains(kinds, Format)
	if !hasLint && !hasFormat {
		return Invocation{}, false
	}
	inv := Invocation{
		Kinds:    kinds,
		Provider: "biome",
		Bin:      "biome",
		Hint:     "run: " + nodePM(p) + " install",
	}
	if mode == CheckMode {
		if hasLint && hasFormat {
			inv.Args = []string{"ci", "."}
		} else if hasLint {
			inv.Args = []string{"lint", "."}
		} else {
			inv.Args = []string{"format", "."}
		}
	} else {
		if hasLint && hasFormat {
			inv.Args = []string{"check", "--write", "."}
		} else if hasLint {
			inv.Args = []string{"lint", "--write", "."}
		} else {
			inv.Args = []string{"format", "--write", "."}
		}
	}
	return inv, true
}

// ESLintAdapter handles eslint.
type eslintAdapter struct{}

func (eslintAdapter) Provider() string { return "eslint" }
func (eslintAdapter) Plan(p Project, e manifest.QualityEntry, kinds []Kind, mode Mode) (Invocation, bool) {
	inv := Invocation{
		Kinds:    kinds,
		Provider: "eslint",
		Bin:      "eslint",
		Hint:     "run: " + nodePM(p) + " install",
	}
	if mode == CheckMode {
		inv.Args = []string{"."}
	} else {
		inv.Args = []string{".", "--fix"}
	}
	return inv, true
}

// PrettierAdapter handles prettier.
type prettierAdapter struct{}

func (prettierAdapter) Provider() string { return "prettier" }
func (prettierAdapter) Plan(p Project, e manifest.QualityEntry, kinds []Kind, mode Mode) (Invocation, bool) {
	inv := Invocation{
		Kinds:    kinds,
		Provider: "prettier",
		Bin:      "prettier",
		Hint:     "run: " + nodePM(p) + " install",
	}
	if mode == CheckMode {
		inv.Args = []string{"--check", "."}
	} else {
		inv.Args = []string{"--write", "."}
	}
	return inv, true
}

// DotnetAdapter handles dotnet build.
type dotnetAdapter struct{}

func (dotnetAdapter) Provider() string { return "dotnet" }
func (dotnetAdapter) Plan(p Project, e manifest.QualityEntry, kinds []Kind, mode Mode) (Invocation, bool) {
	if mode == FixMode {
		return Invocation{}, false
	}
	inv := Invocation{
		Kinds:    kinds,
		Provider: "dotnet",
		Bin:      "dotnet",
		Hint:     "install the .NET SDK: https://dot.net",
	}
	if p.Solution != "" {
		inv.Args = []string{"build", p.Solution, "--nologo"}
	} else {
		inv.Args = []string{"build", "--nologo"}
	}
	return inv, true
}

// DotnetFormatAdapter handles dotnet format.
type dotnetFormatAdapter struct{}

func (dotnetFormatAdapter) Provider() string { return "dotnet-format" }
func (dotnetFormatAdapter) Plan(p Project, e manifest.QualityEntry, kinds []Kind, mode Mode) (Invocation, bool) {
	inv := Invocation{
		Kinds:    kinds,
		Provider: "dotnet-format",
		Bin:      "dotnet",
		Hint:     "install the .NET SDK: https://dot.net",
	}
	if mode == CheckMode {
		if p.Solution != "" {
			inv.Args = []string{"format", p.Solution, "--verify-no-changes"}
		} else {
			inv.Args = []string{"format", "--verify-no-changes"}
		}
	} else {
		if p.Solution != "" {
			inv.Args = []string{"format", p.Solution}
		} else {
			inv.Args = []string{"format"}
		}
	}
	return inv, true
}

// DotnetTestAdapter handles dotnet test.
type dotnetTestAdapter struct{}

func (dotnetTestAdapter) Provider() string { return "dotnet-test" }
func (dotnetTestAdapter) Plan(p Project, e manifest.QualityEntry, kinds []Kind, mode Mode) (Invocation, bool) {
	if mode == FixMode {
		return Invocation{}, false
	}
	trxDir := filepath.ToSlash(filepath.Join(p.TmpDir, "trx"))
	inv := Invocation{
		Kinds:    kinds,
		Provider: "dotnet-test",
		Bin:      "dotnet",
		Report:   trxDir,
		Hint:     "install the .NET SDK: https://dot.net",
	}
	args := []string{"test"}
	if p.Solution != "" {
		args = append(args, p.Solution)
	}
	args = append(args, "--nologo", "--logger", "trx", "--results-directory", trxDir)
	inv.Args = args
	return inv, true
}

// VitestAdapter handles vitest.
type vitestAdapter struct{}

func (vitestAdapter) Provider() string { return "vitest" }
func (vitestAdapter) Plan(p Project, e manifest.QualityEntry, kinds []Kind, mode Mode) (Invocation, bool) {
	if mode == FixMode {
		return Invocation{}, false
	}
	reportFile := filepath.ToSlash(filepath.Join(p.TmpDir, "vitest.json"))
	inv := Invocation{
		Kinds:    kinds,
		Provider: "vitest",
		Bin:      "vitest",
		Report:   reportFile,
		Hint:     "run: " + nodePM(p) + " install",
		Args:     []string{"run", "--reporter=default", "--reporter=json", "--outputFile.json=" + reportFile},
	}
	return inv, true
}

// JestAdapter handles jest.
type jestAdapter struct{}

func (jestAdapter) Provider() string { return "jest" }
func (jestAdapter) Plan(p Project, e manifest.QualityEntry, kinds []Kind, mode Mode) (Invocation, bool) {
	if mode == FixMode {
		return Invocation{}, false
	}
	reportFile := filepath.ToSlash(filepath.Join(p.TmpDir, "jest.json"))
	inv := Invocation{
		Kinds:    kinds,
		Provider: "jest",
		Bin:      "jest",
		Report:   reportFile,
		Hint:     "run: " + nodePM(p) + " install",
		Args:     []string{"--json", "--outputFile=" + reportFile},
	}
	return inv, true
}

// ScriptAdapter handles custom script.
type scriptAdapter struct{}

func (scriptAdapter) Provider() string { return "script" }
func (scriptAdapter) Plan(p Project, e manifest.QualityEntry, kinds []Kind, mode Mode) (Invocation, bool) {
	if mode == FixMode {
		return Invocation{}, false
	}
	pm := nodePM(p)
	inv := Invocation{
		Kinds:    kinds,
		Provider: "script",
		Bin:      pm,
		Hint:     "install " + pm,
		Args:     []string{"run", e.Script},
	}
	return inv, true
}

var adapters = map[string]Adapter{
	"biome":         biomeAdapter{},
	"eslint":        eslintAdapter{},
	"prettier":      prettierAdapter{},
	"dotnet":        dotnetAdapter{},
	"dotnet-format": dotnetFormatAdapter{},
	"dotnet-test":   dotnetTestAdapter{},
	"vitest":        vitestAdapter{},
	"jest":          jestAdapter{},
	"script":        scriptAdapter{},
}

func AdapterFor(provider string) (Adapter, bool) {
	a, ok := adapters[provider]
	return a, ok
}
