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
	"github.com/Ps3udoDev/tija-cli/internal/render"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

var (
	logoStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(render.Orange)).Bold(true)
	catStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color(render.White))
	headingStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(render.White)).Bold(true)
	selectedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(render.Orange)).Bold(true)
	sandStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(render.Sand))
	mutedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(render.Sand)).Faint(true)
	separatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(render.Sand)).Faint(true)
	okStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color(render.Orange))
	warnStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color(render.Sand))
)

// screen is the current step of the UI.
type screen int

const (
	screenDetecting screen = iota // initial detection, scanning mascot
	screenMenu
	screenRunning // an engine command is in flight
	screenChoice  // init: answering a Choice
	screenConsent // init: approving an Action
	screenResult
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

	result string // rendered report
	ok     bool   // the command succeeded
}

// resultMsg carries the rendered outcome of an engine command.
type resultMsg struct {
	title  string
	output string
	ok     bool
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
	case resultMsg:
		m.title = msg.title
		m.result, m.ok = msg.output, msg.ok
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
	case screenDetecting, screenRunning:
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
		return m.consentKey(k)
	case screenResult:
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
	m.flow, m.result = nil, ""
	m.screen = screenDetecting
	return m, detectCmd(m.dir, m.env)
}

// run starts the engine command behind item.
func (m model) run(item MenuItem) (model, tea.Cmd) {
	m.screen, m.title = screenRunning, item.Title
	dir, env := m.dir, m.env
	switch item.Key {
	case keyInit:
		return m, planCmd(dir, env)
	case keyDoctor:
		return m, func() tea.Msg {
			rep, err := engine.Doctor(context.Background(), dir, env)
			if err != nil {
				return failed(item.Title, err)
			}
			return rendered(item.Title, rep.ExitCode() == 0, func(p render.Printer) { render.Doctor(p, rep) })
		}
	default:
		return m, func() tea.Msg {
			rep, err := engine.Status(context.Background(), dir, env)
			if err != nil {
				return failed(item.Title, err)
			}
			return rendered(item.Title, rep.ExitCode(false) == 0, func(p render.Printer) { render.Status(p, rep) })
		}
	}
}

// rendered renders a report in Styled mode into a resultMsg.
func rendered(title string, ok bool, fn func(render.Printer)) resultMsg {
	var buf bytes.Buffer
	fn(render.Printer{W: &buf, Mode: render.Styled})
	return resultMsg{title: title, output: strings.TrimRight(buf.String(), "\n"), ok: ok}
}

func failed(title string, err error) resultMsg {
	return rendered(title, false, func(p render.Printer) { p.Fail(err.Error()) })
}

func (m model) View() tea.View {
	var s string
	switch m.screen {
	case screenDetecting, screenMenu:
		s = m.startupView()
	case screenRunning:
		s = m.panel(Scanning, mutedStyle.Render("Running "+m.title+"..."))
	case screenChoice:
		s = m.panel(Idle, m.choiceView())
	case screenConsent:
		s = m.panel(Idle, m.consentView())
	case screenResult:
		state := Success
		if !m.ok {
			state = Error
		}
		s = m.panel(state, m.result)
	}
	v := tea.NewView(s)
	v.AltScreen = true
	return v
}

// panel is the layout of every screen but the startup one: the top bar,
// the mascot, a titled body and the key help.
func (m model) panel(state MascotState, body string) string {
	content := headingStyle.Render(m.title) + "\n\n" + body
	return topBar(displayPath(m.info.Root), m.version, m.width) + "\n\n" +
		indent(lipgloss.JoinHorizontal(lipgloss.Top, Sprite(state), "   ", content)) +
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
	case screenRunning:
		help = "ctrl+c quit"
	case screenChoice:
		help = "↑/↓ move · enter confirm · esc cancel"
	case screenConsent:
		help = "y apply · n skip · esc cancel"
	case screenResult:
		help = "esc back · q quit"
	}
	return indent(mutedStyle.Render(help))
}

func indent(s string) string {
	return margin + strings.ReplaceAll(s, "\n", "\n"+margin)
}
