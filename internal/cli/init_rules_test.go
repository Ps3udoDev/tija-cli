package cli

import (
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitRulesPresetExplicit(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	code, _, errOut := runIn(t, repo, cmdDeps(false), "", "init", "--yes", "--rules-preset", "dotnet-clean")
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(repo, ".tija/rules/architecture/dotnet-clean.md")); err != nil {
		t.Fatal(err)
	}
	code, _, errOut = runIn(t, repo, cmdDeps(false), "", "init", "--rules-preset", "nope")
	if code != 2 || !strings.Contains(errOut, "react-features") {
		t.Fatalf("%d %s", code, errOut)
	}
}

func TestInitInteractiveDefaultNo(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	code, _, errOut := runIn(t, repo, cmdDeps(true), strings.Repeat("\n", 50), "init")
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(repo, ".tija/rules/architecture/react-features.md")); !os.IsNotExist(err) {
		t.Fatal("empty reply installed preset")
	}
}

func TestInitRulesPresetRecordsExistingManifest(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	if code, _, errOut := runIn(t, repo, cmdDeps(false), "", "init", "--yes"); code != 0 {
		t.Fatal(errOut)
	}
	if code, _, errOut := runIn(t, repo, cmdDeps(false), "", "init", "--rules-preset", "dotnet-clean"); code != 0 {
		t.Fatal(errOut)
	}
	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	if m.Tija.Installed[".tija/rules/architecture/dotnet-clean.md"] == "" {
		t.Fatal("explicit preset missing recorded hash")
	}
}
