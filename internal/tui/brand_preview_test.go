package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestBrandStartupBounds(t *testing.T) {
	for _, width := range []int{60, 80, 100, 120, 160} {
		m := menuModel(t, width)
		if strings.Contains(plainView(m), ">_") || strings.Contains(plainView(m), "( o.o )") {
			t.Fatal("startup must not restore the duplicate character cat")
		}
		firstSpriteLine := strings.Split(Sprite(Idle), "\n")[0]
		if strings.Count(m.View().Content, firstSpriteLine) != 1 {
			t.Fatalf("width %d: expected exactly one colored mascot", width)
		}
		for _, line := range strings.Split(plainView(m), "\n") {
			if lipgloss.Width(line) > width {
				t.Errorf("startup overflows %d columns: %q", width, line)
			}
		}
		if m.View().BackgroundColor == nil || m.View().ForegroundColor == nil {
			t.Fatal("brand canvas must not depend on the user's terminal theme")
		}
	}
}

// Optional visual fixtures, produced from the real views without running an
// agent, a check, or any repository mutation.
func TestBrandPreview(t *testing.T) {
	dir := os.Getenv("TIJA_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set TIJA_PREVIEW_DIR to export visual fixtures")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	m := menuModel(t, 120)
	views := map[string]string{"startup": m.View().Content}
	views["startup-narrow"] = menuModel(t, 60).View().Content
	views["startup-medium"] = menuModel(t, 80).View().Content
	for _, state := range allStates {
		m.screen, m.title = screenRunning, string(state)
		views[string(state)] = m.panel(state, "", "Repository engineering for humans & agents")
	}
	for name, view := range views {
		if err := os.WriteFile(filepath.Join(dir, name+".ansi"), []byte(view), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
