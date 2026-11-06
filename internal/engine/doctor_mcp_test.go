package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

func TestDoctorMCPHintNeverFails(t *testing.T) {
	r, err := Doctor(context.Background(), t.TempDir(), sys.Default())
	if err != nil {
		t.Fatal(err)
	}
	before := r.ExitCode()
	found := false
	for _, c := range r.Checks {
		if c.ID == "mcp" {
			found = true
			if c.Status != StatusOK || !strings.Contains(c.Title, "tija mcp config") {
				t.Fatalf("check=%#v", c)
			}
		}
	}
	if !found {
		t.Fatal("missing informational check")
	}
	checks := r.Checks[:0]
	for _, c := range r.Checks {
		if c.ID != "mcp" {
			checks = append(checks, c)
		}
	}
	r.Checks = checks
	if before != r.ExitCode() {
		t.Fatal("MCP check changed exit code")
	}
}
