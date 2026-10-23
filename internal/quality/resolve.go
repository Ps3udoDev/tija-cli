package quality

import (
	"slices"

	"github.com/Ps3udoDev/tija-cli/internal/detect"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
)

type Source string

const (
	SourceManifest Source = "manifest"
	SourceDetected Source = "detected"
	SourceScript   Source = "script"
)

type Step struct {
	Kind   Kind
	Entry  manifest.QualityEntry
	Source Source
}

// Resolve returns the effective configuration for kinds (nil = all of Order):
// manifest entry if present (provider "none" drops the kind); otherwise DetectedQuality(d);
// otherwise, for Lint and Tests only, a package.json script named "lint" / "test".
func Resolve(m *manifest.Manifest, d detect.Detection, kinds []Kind) []Step {
	targetKinds := kinds
	if targetKinds == nil {
		targetKinds = Order
	}

	detected := manifest.DetectedQuality(d)
	var steps []Step

	for _, k := range targetKinds {
		// Only consider kinds in Order (e2e is ignored)
		if !slices.Contains(Order, k) {
			continue
		}

		// 1. Manifest
		if m != nil && m.Quality != nil {
			if entry, ok := m.Quality[string(k)]; ok {
				if entry.Provider == "none" {
					continue
				}
				steps = append(steps, Step{
					Kind:   k,
					Entry:  entry,
					Source: SourceManifest,
				})
				continue
			}
		}

		// 2. Detected quality
		if entry, ok := detected[string(k)]; ok && entry.Provider != "" && entry.Provider != "none" {
			steps = append(steps, Step{
				Kind:   k,
				Entry:  entry,
				Source: SourceDetected,
			})
			continue
		}

		// 3. Script fallback: only for Lint ("lint") and Tests ("test")
		if k == Lint && slices.Contains(d.Scripts, "lint") {
			steps = append(steps, Step{
				Kind:   Lint,
				Entry:  manifest.QualityEntry{Provider: "script", Script: "lint"},
				Source: SourceScript,
			})
			continue
		}
		if k == Tests && slices.Contains(d.Scripts, "test") {
			steps = append(steps, Step{
				Kind:   Tests,
				Entry:  manifest.QualityEntry{Provider: "script", Script: "test"},
				Source: SourceScript,
			})
			continue
		}
	}

	return steps
}

// Plan groups consecutive steps where supported and produces Invocations with resolved binaries.
func Plan(p Project, steps []Step, mode Mode, lookPath func(string) (string, error)) []Invocation {
	// Group consecutive steps
	type stepGroup struct {
		provider string
		kinds    []Kind
		entry    manifest.QualityEntry
	}

	var groups []stepGroup
	for i := 0; i < len(steps); i++ {
		step := steps[i]
		if len(groups) > 0 {
			prev := &groups[len(groups)-1]
			if prev.provider == step.Entry.Provider && canGroup(prev.provider, prev.kinds, step.Kind) {
				prev.kinds = append(prev.kinds, step.Kind)
				continue
			}
		}
		groups = append(groups, stepGroup{
			provider: step.Entry.Provider,
			kinds:    []Kind{step.Kind},
			entry:    step.Entry,
		})
	}

	var invocations []Invocation
	for _, g := range groups {
		adapter, ok := AdapterFor(g.provider)
		if !ok {
			continue
		}
		inv, ok := adapter.Plan(p, g.entry, g.kinds, mode)
		if !ok {
			continue
		}
		if resolvedPath, found := LookupBin(p.Root, inv.Bin, p.GOOS, lookPath); found {
			inv.Path = resolvedPath
		} else {
			inv.Path = ""
		}
		invocations = append(invocations, inv)
	}

	return invocations
}

func canGroup(provider string, existingKinds []Kind, newKind Kind) bool {
	if provider == "biome" {
		hasLint := slices.Contains(existingKinds, Lint) || newKind == Lint
		hasFormat := slices.Contains(existingKinds, Format) || newKind == Format
		return hasLint && hasFormat && (len(existingKinds) == 1 && (existingKinds[0] == Lint || existingKinds[0] == Format))
	}
	return false
}
