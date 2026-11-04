package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/assets"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

func TestInitOffersPresetForStack(t *testing.T) {
	for _, tc := range []struct{ stack, name string }{{"react-vite", "react-features"}, {"dotnet-api", "dotnet-clean"}, {"unknown", ""}} {
		t.Run(tc.stack, func(t *testing.T) {
			repo := testutil.NewRepo(t, fixture(tc.stack))
			p := plan(t, repo)
			count := 0
			for _, a := range p.Actions {
				if a.OptIn {
					count++
					if a.Path != assets.ArchitectureDir+"/"+tc.name+".md" || !a.NeedsConsent {
						t.Fatalf("offer: %+v", a)
					}
				}
			}
			if (tc.name == "" && count != 0) || (tc.name != "" && count != 1) {
				t.Fatalf("offers %d", count)
			}
			putFile(t, repo, assets.ArchitectureDir+"/custom.md", []byte("custom"))
			for _, a := range plan(t, repo).Actions {
				if a.OptIn {
					t.Fatal("offer with existing rules")
				}
			}
		})
	}
}

func TestInitYesSkipsOptIn(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	p := plan(t, repo)
	approved := map[string]bool{}
	for _, a := range p.Actions {
		if !a.OptIn {
			approved[a.ID] = true
		}
	}
	r := apply(t, p, nil, approved)
	if exists(repo, assets.ArchitectureDir+"/react-features.md") {
		t.Fatal("unapproved preset installed")
	}
	for _, a := range r.Pending {
		if a.OptIn {
			t.Fatal("opt-in remains pending")
		}
	}
}

func TestInitRulesPresetFlag(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	p := plan(t, repo)
	if err := p.AddRulesPresets("dotnet-clean"); err != nil {
		t.Fatal(err)
	}
	apply(t, p, nil, approveAll(p))
	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	rel := assets.ArchitectureDir + "/dotnet-clean.md"
	if !exists(repo, rel) || m.Tija.Installed[rel] == "" {
		t.Fatal("preset/hash missing")
	}
	putFile(t, repo, rel, []byte("user edit"))
	for _, a := range plan(t, repo).Actions {
		if a.Path == rel {
			t.Fatal("edited preset must stay untouched")
		}
	}
	if err := p.AddRulesPresets("nope"); err == nil || !strings.Contains(err.Error(), "react-features") {
		t.Fatalf("invalid preset: %v", err)
	}
}

func TestDoctorArchitectureRules(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("unknown"))
	for _, tc := range []struct {
		text   string
		status CheckStatus
		title  string
	}{{"", StatusOK, "no architecture rules (optional)"}, {"---\nname: valid\n---\n## own-rule: Title\nBody", StatusOK, "1 valid files"}, {"---\nname: bad\nchecks:\n - id: bad-regex\n   severity: High\n   forbid: '['\n---", StatusFail, "bad-regex"}} {
		if tc.text != "" {
			putFile(t, repo, assets.ArchitectureDir+"/rules.md", []byte(tc.text))
		}
		r, err := Doctor(context.Background(), repo, fakeEnv())
		if err != nil {
			t.Fatal(err)
		}
		c, ok := checkByID(r, "architecture.rules")
		if !ok || c.Status != tc.status || !strings.Contains(c.Title, tc.title) {
			t.Fatalf("check: %+v", c)
		}
	}
}
