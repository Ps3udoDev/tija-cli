package engine

import (
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
	"strings"
	"testing"
)

func TestInitProposesReviewSection(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	p := plan(t, repo)
	apply(t, p, nil, approveAll(p))
	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	m.Review = manifest.Review{}
	if err := manifest.Save(repo, m); err != nil {
		t.Fatal(err)
	}
	p = plan(t, repo)
	found := false
	for _, a := range p.Actions {
		if a.Path == manifest.Path && strings.Contains(a.Diff, "+review:") {
			found = true
		}
	}
	if !found {
		t.Fatal("review not proposed")
	}
	apply(t, p, nil, approveAll(p))
	for _, a := range plan(t, repo).Actions {
		if a.Path == manifest.Path && strings.Contains(a.Diff, "+review:") {
			t.Fatal("review proposed twice")
		}
	}
}
