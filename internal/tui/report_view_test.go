package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/charmbracelet/x/ansi"
)

func reportModel(t *testing.T) model {
	t.Helper()
	m := newModel("/x", "dev", sys.Env{})
	m.now = func() time.Time { return time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC) }
	m, _ = m.startReport()
	return m
}

func press(t *testing.T, m model, keys ...string) model {
	t.Helper()
	for _, k := range keys {
		m, _ = m.handleKey(k)
	}
	return m
}

func TestReportPresets(t *testing.T) {
	for _, tt := range []struct {
		today time.Time
		title string
		since string
		until string
	}{
		{time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC), "This month", "2026-11-01", "2026-11-15"},
		{time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC), "Last month", "2026-10-01", "2026-10-31"},
		{time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), "Last month", "2026-02-01", "2026-02-28"},
		{time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC), "Last month", "2025-12-01", "2025-12-31"},
		{time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC), "Last 7 days", "2026-10-28", "2026-11-03"},
	} {
		i := slices.IndexFunc(reportPresets(tt.today), func(p reportPreset) bool { return p.Title == tt.title })
		p := reportPresets(tt.today)[i]
		if p.Since != tt.since || p.Until != tt.until {
			t.Errorf("%s on %s = %s..%s, want %s..%s", tt.title, tt.today.Format(dateLayout), p.Since, p.Until, tt.since, tt.until)
		}
	}
}

func TestReportChoosesPreset(t *testing.T) {
	m := reportModel(t)
	if got := ansi.Strip(m.reportBody()); !strings.Contains(got, "2026-10-01 to 2026-10-31") || !strings.Contains(got, "Custom range") {
		t.Errorf("body = %q", got)
	}
	m = press(t, m, "down")
	m, cmd := m.handleKey("enter")
	if cmd == nil || m.report.state != reportWorking {
		t.Fatalf("state = %v, cmd = %v", m.report.state, cmd)
	}
	if m.report.since != "2026-10-01" || m.report.until != "2026-10-31" {
		t.Errorf("period = %s..%s", m.report.since, m.report.until)
	}
}

func TestReportCustomRange(t *testing.T) {
	m := press(t, reportModel(t), "down", "down", "down", "enter")
	if m.report.state != reportEditing || m.report.since != "2026-11-01" || m.report.until != "2026-11-15" {
		t.Fatalf("flow = %+v", m.report)
	}

	// Letters are ignored, and a date cannot grow past its format.
	m = press(t, m, "backspace", "backspace", "x", "q", "1", "5", "9")
	if m.report.since != "2026-11-15" {
		t.Errorf("since = %q", m.report.since)
	}
	if m.screen != screenReport {
		t.Errorf("q must not leave the screen while typing: %v", m.screen)
	}

	// Enter moves to the second date, and an invalid range stays on screen.
	m = press(t, m, "enter", "backspace", "backspace", "0", "1", "enter")
	if !m.report.editUntil || m.report.state != reportEditing || !strings.Contains(m.report.err, "before") {
		t.Fatalf("flow = %+v", m.report)
	}
	if !strings.Contains(ansi.Strip(m.reportBody()), "before") {
		t.Errorf("body misses the error: %q", ansi.Strip(m.reportBody()))
	}

	// Typing clears the error; a valid range runs.
	m = press(t, m, "backspace", "backspace", "1", "5")
	if m.report.err != "" {
		t.Errorf("err = %q", m.report.err)
	}
	m, cmd := m.handleKey("enter")
	if cmd == nil || m.report.state != reportWorking {
		t.Fatalf("state = %v", m.report.state)
	}
}

func TestReportEscGoesBack(t *testing.T) {
	m := press(t, reportModel(t), "down", "down", "down", "enter", "esc")
	if m.report.state != reportChoosing {
		t.Errorf("esc from the form: state = %v", m.report.state)
	}
	m = press(t, m, "esc")
	if m.screen != screenDetecting || m.report != nil {
		t.Errorf("esc from presets: screen = %v, report = %v", m.screen, m.report)
	}
}

func TestReportWorkingShowsStages(t *testing.T) {
	m := reportModel(t)
	m.report.state, m.report.since, m.report.until = reportWorking, "2026-10-01", "2026-10-31"
	next, _ := m.Update(reportEventMsg(engine.ReportEvent{Stage: "draft"}))
	body := ansi.Strip(next.(model).reportBody())
	for _, want := range []string{"2026-10-01 to 2026-10-31", "✓ Collect commits", "> Write the report", "· Generate the PDF"} {
		if !strings.Contains(body, want) {
			t.Errorf("body misses %q: %q", want, body)
		}
	}
}

func TestReportKeysIgnoredWhileWorking(t *testing.T) {
	m := reportModel(t)
	m.report.state = reportWorking
	m = press(t, m, "esc", "q", "enter")
	if m.screen != screenReport || m.report == nil {
		t.Errorf("screen = %v", m.screen)
	}
}

func TestReportMenuItem(t *testing.T) {
	has := func(items []MenuItem) bool {
		return slices.ContainsFunc(items, func(it MenuItem) bool { return it.Key == keyReport })
	}
	if has(MenuItems(true, false, false, false)) {
		t.Error("the item must stay hidden without the packs")
	}
	got := titles(MenuItems(true, false, false, true))
	want := []string{"Pull Request", "Check", "Tests", "Activity report", "Doctor", "Status", "Init"}
	if !slices.Equal(got, want) {
		t.Errorf("items = %v, want %v", got, want)
	}
}
