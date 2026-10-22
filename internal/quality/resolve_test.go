package quality_test

import (
	"reflect"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/detect"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
)

func TestResolvePrecedence(t *testing.T) {
	// 1. Manifest lint biome + detection eslint -> biome (Source manifest)
	m := &manifest.Manifest{
		Quality: map[string]manifest.QualityEntry{
			"lint": {Provider: "biome"},
		},
	}
	d := detect.Detection{Tools: []string{"eslint"}}
	steps := quality.Resolve(m, d, nil)
	if len(steps) != 1 || steps[0].Kind != quality.Lint || steps[0].Entry.Provider != "biome" || steps[0].Source != "manifest" {
		t.Fatalf("step 0 = %+v", steps)
	}

	// 2. Manifest tests none -> no tests step
	mNone := &manifest.Manifest{
		Quality: map[string]manifest.QualityEntry{
			"tests": {Provider: "none"},
		},
	}
	stepsNone := quality.Resolve(mNone, detect.Detection{Tools: []string{"vitest"}}, nil)
	for _, s := range stepsNone {
		if s.Kind == quality.Tests {
			t.Fatalf("tests step should not exist when provider is none: %+v", s)
		}
	}

	// 3. No manifest, Scripts ["lint", "test"] without detected tools -> lint and tests "script" (Source script); format never uses script
	dScripts := detect.Detection{Scripts: []string{"build", "lint", "test"}}
	stepsScript := quality.Resolve(nil, dScripts, nil)
	var scriptKinds []quality.Kind
	for _, s := range stepsScript {
		if s.Source == "script" {
			scriptKinds = append(scriptKinds, s.Kind)
			if s.Kind == quality.Format {
				t.Fatalf("format should never use script fallback")
			}
		}
	}
	wantKinds := []quality.Kind{quality.Lint, quality.Tests}
	if !reflect.DeepEqual(scriptKinds, wantKinds) {
		t.Fatalf("scriptKinds = %v, want %v", scriptKinds, wantKinds)
	}
}

func TestResolveKindsFilter(t *testing.T) {
	m := &manifest.Manifest{
		Quality: map[string]manifest.QualityEntry{
			"lint":  {Provider: "biome"},
			"tests": {Provider: "vitest"},
		},
	}
	steps := quality.Resolve(m, detect.Detection{}, []quality.Kind{quality.Tests})
	if len(steps) != 1 || steps[0].Kind != quality.Tests {
		t.Fatalf("filtered steps = %+v, want only tests", steps)
	}
}

func TestResolveIgnoresE2E(t *testing.T) {
	m := &manifest.Manifest{
		Quality: map[string]manifest.QualityEntry{
			"e2e": {Provider: "playwright"},
		},
	}
	steps := quality.Resolve(m, detect.Detection{}, nil)
	for _, s := range steps {
		if s.Kind == "e2e" {
			t.Fatalf("e2e step should be ignored in phase 2: %+v", s)
		}
	}
}
