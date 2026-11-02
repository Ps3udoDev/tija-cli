package tui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/render"
)

const (
	reportTitle  = "Activity report"
	dateLayout   = "2006-01-02"
	customPreset = "Custom range"
)

type reportState int

const (
	reportChoosing reportState = iota // picking a preset
	reportEditing                     // typing a custom range
	reportWorking
)

// reportPreset is one ready-made period; a custom range has no dates.
type reportPreset struct {
	Title, Since, Until string
}

// reportStages are the steps shown while a report is generated.
var reportStages = []struct{ Key, Title string }{
	{"collect", "Collect commits and pull requests"},
	{"draft", "Write the report"},
	{"build", "Generate the PDF"},
}

type reportFlow struct {
	state   reportState
	presets []reportPreset
	cursor  int

	since, until string
	editUntil    bool // the until field has the focus
	err          string

	stage  string
	events chan engine.ReportEvent
}

type reportEventMsg engine.ReportEvent

func waitForReportEvent(ch <-chan engine.ReportEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return reportEventMsg(ev)
	}
}

// reportPresets returns the periods offered on today. A period in progress
// ends today, since later work does not exist yet.
func reportPresets(today time.Time) []reportPreset {
	first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
	return []reportPreset{
		{"This month", first.Format(dateLayout), today.Format(dateLayout)},
		{"Last month", first.AddDate(0, -1, 0).Format(dateLayout), first.AddDate(0, 0, -1).Format(dateLayout)},
		{"Last 7 days", today.AddDate(0, 0, -6).Format(dateLayout), today.Format(dateLayout)},
		{Title: customPreset},
	}
}

func (m model) today() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}

func (m model) startReport() (model, tea.Cmd) {
	m.screen, m.title, m.mascot, m.caption = screenReport, reportTitle, Idle, ""
	m.report = &reportFlow{presets: reportPresets(m.today())}
	return m, nil
}

func (m model) reportKey(k string) (model, tea.Cmd) {
	r := m.report
	if r == nil {
		return m.backToMenu()
	}
	switch r.state {
	case reportChoosing:
		switch k {
		case "up", "k":
			r.cursor = max(r.cursor-1, 0)
		case "down", "j":
			r.cursor = min(r.cursor+1, len(r.presets)-1)
		case "enter":
			p := r.presets[r.cursor]
			if p.Since != "" {
				return m.runReport(p.Since, p.Until)
			}
			// Start from this month: changing a date beats typing two.
			r.state, r.editUntil, r.err = reportEditing, false, ""
			r.since, r.until = r.presets[0].Since, r.presets[0].Until
		case "esc", "q":
			return m.backToMenu()
		}
	case reportEditing:
		return m.reportEditKey(k)
	}
	return m, nil
}

func (m model) reportEditKey(k string) (model, tea.Cmd) {
	r := m.report
	field := &r.since
	if r.editUntil {
		field = &r.until
	}
	switch k {
	case "esc":
		r.state, r.err = reportChoosing, ""
	case "tab", "shift+tab", "up", "down":
		r.editUntil = !r.editUntil
	case "backspace":
		if *field != "" {
			*field = (*field)[:len(*field)-1]
		}
		r.err = ""
	case "enter":
		if !r.editUntil {
			r.editUntil = true
			return m, nil
		}
		if err := engine.ValidatePeriod(r.since, r.until, m.today()); err != nil {
			r.err = err.Error()
			return m, nil
		}
		return m.runReport(r.since, r.until)
	default:
		if len(k) == 1 && (k == "-" || (k[0] >= '0' && k[0] <= '9')) && len(*field) < len(dateLayout) {
			*field += k
			r.err = ""
		}
	}
	return m, nil
}

func (m model) runReport(since, until string) (model, tea.Cmd) {
	r := m.report
	r.state, r.since, r.until, r.stage = reportWorking, since, until, ""
	m.mascot = Scanning
	ch := make(chan engine.ReportEvent, 8)
	r.events = ch
	dir, env := m.dir, m.env
	cmd := func() tea.Msg {
		defer close(ch)
		res, err := engine.ActivityReport(context.Background(), dir, env, engine.ReportRequest{Since: since, Until: until}, func(ev engine.ReportEvent) {
			ch <- ev
		})
		if err != nil {
			return genericFailure(failed(reportTitle, err))
		}
		mascot := Success
		if res.PDF == "" {
			mascot = Warning
		}
		return rendered(reportTitle, mascot, func(p render.Printer) { render.ActivityReport(p, res) })
	}
	return m, tea.Batch(cmd, waitForReportEvent(ch))
}

func (m model) applyReportEvent(ev engine.ReportEvent) {
	if m.report != nil {
		m.report.stage = ev.Stage
	}
}

// reportMascot follows the stage: the agent thinks, the rest scans.
func (m model) reportMascot() MascotState {
	if m.report != nil && m.report.state == reportWorking && m.report.stage == "draft" {
		return Thinking
	}
	return m.mascot
}

func (m model) reportBody() string {
	r := m.report
	if r == nil {
		return ""
	}
	switch r.state {
	case reportEditing:
		return m.reportEditBody()
	case reportWorking:
		return m.reportWorkingBody()
	}
	items := make([]MenuItem, len(r.presets))
	for i, p := range r.presets {
		items[i] = MenuItem{Key: p.Title, Title: p.Title, Description: "Choose the two dates"}
		if p.Since != "" {
			items[i].Description = p.Since + " to " + p.Until
		}
	}
	return headingStyle.Render("Which period?") + "\n\n" + renderMenu(items, r.cursor, true)
}

func (m model) reportEditBody() string {
	r := m.report
	row := func(label, value string, focused bool) string {
		if focused {
			return selectedStyle.Render("> ") + itemTitleStyle.Render(label) + "  " + selTitleStyle.Render(value+"_")
		}
		return "  " + mutedStyle.Render(label) + "  " + value
	}
	lines := []string{
		headingStyle.Render("Custom range"), "",
		row("From", r.since, !r.editUntil),
		row("To  ", r.until, r.editUntil),
		"", mutedStyle.Render("Format: YYYY-MM-DD"),
	}
	if r.err != "" {
		lines = append(lines, "", failStyle.Render(r.err))
	}
	return strings.Join(lines, "\n")
}

func (m model) reportWorkingBody() string {
	r := m.report
	lines := []string{mutedStyle.Render(r.since + " to " + r.until), ""}
	current := -1
	for i, s := range reportStages {
		if s.Key == r.stage {
			current = i
		}
	}
	for i, s := range reportStages {
		sym := mutedStyle.Render("·")
		switch {
		case i < current:
			sym = okStyle.Render("✓")
		case i == current:
			sym = selectedStyle.Render(">")
		}
		lines = append(lines, sym+" "+s.Title)
	}
	return strings.Join(lines, "\n")
}
