package rules_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/assets"
	"github.com/Ps3udoDev/tija-cli/internal/rules"
)

const sample = "---\nname: backend-dotnet\nappliesTo: [\"src/**/*.cs\"]\nchecks:\n  - id: be-domain-no-infra\n    severity: high\n    files: \"src/*.Domain/**/*.cs\"\n    forbid: 'Infrastructure'\n    message: No infrastructure\n---\n## be-thin-controllers: Controladores delgados\nTexto libre.\n"

func write(t *testing.T, root, name, text string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}
func load(t *testing.T, text string, reserved []string) (rules.Set, []error) {
	t.Helper()
	root := t.TempDir()
	write(t, root, "rules/a.md", text)
	return rules.Load(root, []string{"rules/*.md"}, reserved)
}
func TestLoadRuleFile(t *testing.T) {
	set, errs := load(t, strings.ReplaceAll(sample, "\n", "\r\n"), nil)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	f := set.Files[0]
	if f.Name != "backend-dotnet" || len(f.Checks) != 1 || f.Checks[0].Severity != "High" || len(f.Rules) != 1 || f.Rules[0].Title != "Controladores delgados" || f.Rules[0].Text != "Texto libre." {
		t.Fatalf("%+v", f)
	}
}
func TestLoadNoFiles(t *testing.T) {
	set, errs := rules.Load(t.TempDir(), []string{".tija/rules/architecture/*.md"}, nil)
	if len(errs) > 0 || len(set.Files) > 0 {
		t.Fatalf("%v %v", set, errs)
	}
}
func TestLoadErrors(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		reserved   []string
		contains   string
	}{
		{"regex", strings.ReplaceAll(sample, "'Infrastructure'", "'['"), nil, "be-domain-no-infra"},
		{"severity", strings.ReplaceAll(sample, "severity: high", "severity: Urgent"), nil, "be-domain-no-infra"},
		{"reserved", sample, []string{"be-domain-no-infra"}, "be-domain-no-infra"},
		{"name", strings.ReplaceAll(sample, "name: backend-dotnet", "name: ''"), nil, "name"},
		{"yaml", strings.ReplaceAll(sample, "name: backend-dotnet", "name: ["), nil, "yaml"},
		{"duplicate", sample + "\n## be-domain-no-infra: Duplicate\nBody", nil, "be-domain-no-infra"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := load(t, tc.text, tc.reserved)
			if len(errs) == 0 || !strings.Contains(errs[0].Error(), "rules/a.md") || !strings.Contains(errs[0].Error(), tc.contains) {
				t.Fatalf("%v", errs)
			}
		})
	}
}
func TestLoadDuplicateAcrossFiles(t *testing.T) {
	root := t.TempDir()
	write(t, root, "rules/a.md", sample)
	write(t, root, "rules/b.md", strings.ReplaceAll(sample, "backend-dotnet", "other"))
	_, errs := rules.Load(root, []string{"rules/*.md"}, nil)
	if len(errs) != 2 {
		t.Fatalf("%v", errs)
	}
}
func TestRunChecksAddedLinesOnly(t *testing.T) {
	set, _ := load(t, sample, nil)
	got := set.Run([]rules.Changed{{Path: "src/Api.Domain/Order.cs", Added: map[int]string{8: "using Infrastructure;", 9: "safe"}}})
	if len(got) != 1 || got[0].Line != 8 || got[0].Text != "using Infrastructure;" {
		t.Fatalf("%+v", got)
	}
}
func TestRunChecksWindowsPaths(t *testing.T) {
	set, _ := load(t, sample, nil)
	got := set.Run([]rules.Changed{{Path: `src\Api.Domain\Order.cs`, Added: map[int]string{4: "Infrastructure"}}})
	if len(got) != 1 || got[0].File != "src/Api.Domain/Order.cs" {
		t.Fatalf("%+v", got)
	}
}
func TestAgentRulesAppliesTo(t *testing.T) {
	set, _ := load(t, sample, nil)
	if len(set.AgentRules([]string{"src/a.cs"})) != 1 || len(set.AgentRules([]string{"web/a.tsx"})) != 0 {
		t.Fatal("appliesTo ignored")
	}
}
func TestPresetsEmbedded(t *testing.T) {
	if !reflect.DeepEqual(assets.Presets(), []string{"dotnet-clean", "react-features"}) {
		t.Fatal(assets.Presets())
	}
	root := t.TempDir()
	for _, name := range assets.Presets() {
		data, ok := assets.Preset(name)
		if !ok {
			t.Fatal(name)
		}
		write(t, root, assets.ArchitectureDir+"/"+name+".md", string(data))
	}
	set, errs := rules.Load(root, []string{assets.ArchitectureDir + "/*.md"}, nil)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	for _, f := range set.Files {
		if len(f.Checks) < 2 || len(f.Rules) < 3 {
			t.Fatalf("%+v", f)
		}
	}
	for p := range assets.Files() {
		if strings.HasPrefix(p, assets.ArchitectureDir+"/") {
			t.Fatal(p)
		}
	}
	if _, ok := assets.Preset("../dotnet-clean"); ok {
		t.Fatal("path traversal")
	}
}
func TestPresetSeededViolations(t *testing.T) {
	root := t.TempDir()
	for _, name := range assets.Presets() {
		b, _ := assets.Preset(name)
		write(t, root, "rules/"+name+".md", string(b))
	}
	set, errs := rules.Load(root, []string{"rules/*.md"}, nil)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	got := set.Run([]rules.Changed{
		{Path: "src/App.Domain/Order.cs", Added: map[int]string{1: "using App.Infrastructure;"}},
		{Path: "src/App.Api/Controllers/OrdersController.cs", Added: map[int]string{1: "AppDbContext db;"}},
		{Path: "src/components/Orders.tsx", Added: map[int]string{1: "fetch('/orders');"}},
		{Path: "src/features/orders/services/api.ts", Added: map[int]string{1: "import {x} from '@/features/payments/api';"}},
	})
	if len(got) != 4 {
		t.Fatalf("seeded 4 violations, got %+v", got)
	}
}
