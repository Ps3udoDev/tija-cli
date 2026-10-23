// Package tui is the interactive terminal UI of Tija, started by running
// tija with no arguments in a terminal.
package tui

import (
	"bytes"
	"context"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
	"github.com/Ps3udoDev/tija-cli/internal/render"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

const (
	terminalBackground  = "#081823"
	selectionBackground = "#1E3A50"
)

var (
	logoStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(render.Orange)).Bold(true)
	headingStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(render.White)).Bold(true)
	selectedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(render.Orange)).Bold(true)
	sandStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(render.Sand))
	mutedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#A8B8C5"))
	separatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#354957"))
	okStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color(render.Green))
	failStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(render.Orange)).Bold(true)
	warnStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(render.Sand))
)

// screen is the current step of the UI.
type screen int

const (
	screenDetecting screen = iota // initial detection, scanning mascot
	screenMenu
	screenRunning // an engine command is in flight
	screenQuality // live quality check or tests running
	screenChoice  // init: answering a Choice
	screenConsent // init: approving an Action; pull request: approving the plan
	screenResult
	screenPRWorking // pull request: planning and drafting
	screenPRPreview // pull request: draft and validation
)

// defaultWidth is assumed until the terminal reports its size.
const defaultWidth = wideLayout

// margin indents the body of every screen.
const margin = "  "

type model struct {
	dir     string
	version string
	env     sys.Env

	width, height int
	screen        screen

	info   startupInfo
	items  []MenuItem
	cursor int

	title string    // title of the current command
	flow  *initFlow // state of an interactive init

	result  string      // rendered report
	ok      bool        // the command succeeded
	mascot  MascotState // mascot expression in result
	caption string      // overrides Caption(mascot) in result when set

	// Scrolling
	scrollOffset    int
	maxScrollOffset int
	needsScroll     bool

	// Quality live progress
	qualitySteps  []qualityStepState
	spinnerIndex  int
	qualityEvents chan engine.QualityEvent

	// Pull Request flow
	pr       *prFlow
	prEvents chan engine.PREvent
}

// resultMsg carries the rendered outcome of an engine command.
type resultMsg struct {
	title  string
	output string
	mascot MascotState
	// caption overrides Caption(mascot) when not empty.
	caption string
	ok      bool
}

func newModel(dir, version string, env sys.Env) model {
	return model{dir: dir, version: version, env: env, width: defaultWidth, screen: screenDetecting}
}

// Run starts the TUI for the repository containing dir.
func Run(dir, version string, env sys.Env) error {
	_, err := tea.NewProgram(newModel(dir, version, env)).Run()
	return err
}

func (m model) Init() tea.Cmd { return detectCmd(m.dir, m.env) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg.String())
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case startupMsg:
		m.info = msg.info
		m.items = MenuItems(msg.info.Initialized)
		m.cursor = 0
		m.screen = screenMenu
	case planMsg:
		return m.startFlow(msg)
	case qualityEventMsg:
		m.applyQualityEvent(engine.QualityEvent(msg))
		if m.qualityEvents != nil {
			return m, waitForEvent(m.qualityEvents)
		}
	case spinnerTickMsg:
		if m.screen == screenQuality || m.screen == screenPRWorking {
			m.spinnerIndex++
			return m, tickSpinner()
		}
	case prEventMsg, prPlanMsg, prEditorMsg, prEditedMsg, prCreatedMsg:
		return m.updatePR(msg)
	case resultMsg:
		m.title = msg.title
		m.result, m.ok, m.mascot, m.caption = msg.output, msg.ok, msg.mascot, msg.caption
		if m.mascot == "" {
			if m.ok {
				m.mascot = Success
			} else {
				m.mascot = Error
			}
		}
		m.scrollOffset = 0
		m.flow = nil
		m.screen = screenResult
	}
	return m, nil
}

// handleKey reacts to a key given as tea.KeyPressMsg.String().
func (m model) handleKey(k string) (model, tea.Cmd) {
	if k == "ctrl+c" {
		return m, tea.Quit
	}
	switch m.screen {
	case screenDetecting, screenRunning, screenQuality, screenPRWorking:
		if k == "q" && m.screen == screenDetecting {
			return m, tea.Quit
		}
	case screenMenu:
		switch k {
		case "up", "k":
			m.cursor = max(m.cursor-1, 0)
		case "down", "j":
			m.cursor = min(m.cursor+1, len(m.items)-1)
		case "enter":
			return m.run(m.items[m.cursor])
		case "q":
			return m, tea.Quit
		}
	case screenChoice:
		return m.choiceKey(k)
	case screenConsent:
		if m.flow == nil && m.pr != nil {
			return m.prConsentKey(k)
		}
		return m.consentKey(k)
	case screenPRPreview:
		return m.prPreviewKey(k)
	case screenResult:
		if m.scroll(k, m.result, m.height) {
			return m, nil
		}
		switch k {
		case "esc":
			return m.backToMenu()
		case "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

// backToMenu recomputes the startup info, since a command such as Init may
// have changed the repository.
func (m model) backToMenu() (model, tea.Cmd) {
	m.flow, m.pr, m.result = nil, nil, ""
	m.screen = screenDetecting
	return m, detectCmd(m.dir, m.env)
}

// run starts the engine command behind item.
func (m model) run(item MenuItem) (model, tea.Cmd) {
	dir, env := m.dir, m.env
	switch item.Key {
	case keyPR:
		m.pr = nil
		return m.startPR(false)

	case keyCheck, keyTests:
		m.screen = screenQuality
		m.title = item.Title
		m.spinnerIndex = 0
		m.qualitySteps = nil
		ch := make(chan engine.QualityEvent, 32)
		m.qualityEvents = ch
		var kinds []quality.Kind
		if item.Key == keyTests {
			kinds = []quality.Kind{quality.Tests}
		}
		req := engine.QualityRequest{
			Kinds: kinds,
			Mode:  quality.CheckMode,
		}
		cmd := func() tea.Msg {
			defer close(ch)
			rep, err := engine.RunQuality(context.Background(), dir, env, req, func(ev engine.QualityEvent) {
				ch <- ev
			})
			if err != nil {
				return failed(item.Title, err)
			}
			mascot := Success
			if rep.ExitCode() != 0 {
				mascot = Warning
			}
			return rendered(item.Title, mascot, func(p render.Printer) { render.Quality(p, rep) })
		}
		return m, tea.Batch(cmd, waitForEvent(ch), tickSpinner())

	case keyInit:
		m.screen, m.title = screenRunning, item.Title
		return m, planCmd(dir, env)

	case keyDoctor:
		m.screen, m.title = screenRunning, item.Title
		return m, func() tea.Msg {
			rep, err := engine.Doctor(context.Background(), dir, env)
			if err != nil {
				return genericFailure(failed(item.Title, err))
			}
			mascot := Success
			hasWarn := false
			for _, c := range rep.Checks {
				if c.Status == engine.StatusWarn {
					hasWarn = true
				}
			}
			if rep.ExitCode() != 0 {
				mascot = Error
			} else if hasWarn {
				mascot = Warning
			}
			return genericFailure(rendered(item.Title, mascot, func(p render.Printer) { render.Doctor(p, rep) }))
		}

	default: // status
		m.screen, m.title = screenRunning, item.Title
		return m, func() tea.Msg {
			rep, err := engine.Status(context.Background(), dir, env)
			if err != nil {
				return genericFailure(failed(item.Title, err))
			}
			mascot := Success
			if rep.ExitCode(false) != 0 {
				mascot = Error
			}
			return genericFailure(rendered(item.Title, mascot, func(p render.Printer) { render.Status(p, rep) }))
		}
	}
}

// rendered renders a report in Styled mode into a resultMsg.
func rendered(title string, mascot MascotState, fn func(render.Printer)) resultMsg {
	var buf bytes.Buffer
	fn(render.Printer{W: &buf, Mode: render.Styled})
	return resultMsg{
		title:  title,
		output: strings.TrimRight(buf.String(), "\n"),
		mascot: mascot,
		ok:     mascot == Success,
	}
}

func failed(title string, err error) resultMsg {
	return rendered(title, Error, func(p render.Printer) { p.Fail(err.Error()) })
}

// genericFailureCaption replaces "Check failed." for commands that are not a
// quality check, such as Doctor and Status.
const genericFailureCaption = "Something failed."

// genericFailure gives an Error result the generic failure caption.
func genericFailure(msg resultMsg) resultMsg {
	if msg.mascot == Error {
		msg.caption = genericFailureCaption
	}
	return msg
}

func (m model) View() tea.View {
	var s string
	switch m.screen {
	case screenDetecting, screenMenu:
		s = m.startupView()
	case screenRunning:
		s = m.panel(Scanning, "", mutedStyle.Render("Running "+m.title+"..."))
	case screenQuality:
		s = m.panel(Scanning, "", m.qualityView())
	case screenChoice:
		s = m.panel(Idle, "", m.choiceView())
	case screenConsent:
		s = m.panel(Idle, "", m.consentBody())
	case screenPRWorking:
		state, body := m.prWorkingView()
		s = m.panel(state, "", body)
	case screenPRPreview:
		state, caption, body := m.prPreviewView()
		s = m.panel(state, caption, body)
	case screenResult:
		body, _, _ := wrapAndSliceResult(m.result, m.width, m.height, m.scrollOffset)
		s = m.panel(m.mascot, m.caption, body)
	}
	v := tea.NewView(s)
	v.BackgroundColor = lipgloss.Color(terminalBackground)
	v.ForegroundColor = lipgloss.Color(render.White)
	v.AltScreen = true
	return v
}

// mascotColumnWidth is the width of the panel column holding the mascot and
// its caption and accessory. A fixed width keeps the body still between states.
const mascotColumnWidth = 26

// panelGap separates the mascot column from the body.
const panelGap = "   "

// panelBodyIndent is the column where the body of a panel starts.
const panelBodyIndent = len(margin) + mascotColumnWidth + len(panelGap)

// panel is the layout of every screen but the startup one: the top bar,
// the mascot with its caption, a titled body and the key help. An empty
// caption uses Caption(state).
func (m model) panel(state MascotState, caption, body string) string {
	if caption == "" {
		caption = Caption(state)
	}
	mascot := lipgloss.NewStyle().Width(mascotColumnWidth).
		Render(Sprite(state) + "\n\n" + mutedStyle.Render(caption))
	content := headingStyle.Render(m.title) + "\n\n" + body
	return topBar(displayPath(m.info.Root), m.version, m.width) + "\n\n" +
		indent(lipgloss.JoinHorizontal(lipgloss.Top, mascot, panelGap, content)) +
		"\n\n" + m.footer()
}

// footer is the key help of the current screen.
func (m model) footer() string {
	var help string
	switch m.screen {
	case screenDetecting:
		help = "q quit"
	case screenMenu:
		help = "↑/↓ move · enter select · q quit"
	case screenRunning, screenQuality, screenPRWorking:
		help = "ctrl+c quit"
	case screenChoice:
		help = "↑/↓ move · enter confirm · esc cancel"
	case screenConsent:
		help = "y apply · n skip · esc cancel"
		if m.flow == nil && m.pr != nil {
			help = "y create · n back"
		}
	case screenPRPreview:
		help = "↑/↓ scroll · e edit · r regenerate · enter create · esc back"
	case screenResult:
		_, needsScroll, _ := wrapAndSliceResult(m.result, m.width, m.height, 0)
		if needsScroll {
			help = "↑/↓ scroll · esc back · q quit"
		} else {
			help = "esc back · q quit"
		}
	}
	// The brand tagline sits at the right edge when it fits with a gap of
	// at least two cells.
	gap := m.width - len(margin) - lipgloss.Width(help) - lipgloss.Width(brandTagline)
	if gap >= 2 {
		return indent(mutedStyle.Render(help) + strings.Repeat(" ", gap) + mutedStyle.Render(brandTagline))
	}
	return indent(mutedStyle.Render(help))
}

func indent(s string) string {
	return margin + strings.ReplaceAll(s, "\n", "\n"+margin)
}
