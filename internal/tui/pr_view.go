package tui

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/render"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

// prFlow is the state of the Pull Request flow: drafting, preview, consent.
type prFlow struct {
	req     engine.CreateRequest
	plan    engine.CreatePlan
	stage   engine.PRStage // stage of the last PREvent
	message string         // message of the last PREvent
	agent   string         // agent drafting, from the last agent event
	hint    string         // note under the preview, such as why e did nothing
}

// prEventMsg is a progress event of PlanCreatePR or PreparePR.
type prEventMsg engine.PREvent

// prPlanMsg delivers the result of PlanCreatePR.
type prPlanMsg struct {
	plan engine.CreatePlan
	err  error
}

// prCreatedMsg delivers the result of ApplyCreatePR.
type prCreatedMsg struct {
	rep engine.CreateReport
	err error
}

// prEditorMsg carries the editor command resolved from git var GIT_EDITOR.
type prEditorMsg struct {
	cmd *exec.Cmd
	err error
}

// prEditedMsg arrives when the editor exits.
type prEditedMsg struct{ err error }

func waitForPREvent(ch <-chan engine.PREvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return prEventMsg(ev)
	}
}

// startPR plans the pull request, streaming its events to the working
// screen. With regenerate, a saved draft is replaced first: PreparePR
// drafts and saves a new one, which PlanCreatePR then reuses. Without a
// saved draft PlanCreatePR always drafts anew, so it is enough.
func (m model) startPR(regenerate bool) (model, tea.Cmd) {
	if m.pr == nil {
		m.pr = &prFlow{}
	}
	f := m.pr
	f.stage, f.message, f.agent, f.hint = engine.StageContext, "", "", ""
	m.screen, m.title = screenPRWorking, itemPR.Title
	m.qualitySteps, m.spinnerIndex, m.scrollOffset = nil, 0, 0
	ch := make(chan engine.PREvent, 32)
	m.prEvents = ch
	dir, env, req := m.dir, m.env, f.req
	prepare := regenerate && f.plan.Report.DraftPath != ""
	cmd := func() tea.Msg {
		defer close(ch)
		ctx := context.Background()
		emit := func(e engine.PREvent) { ch <- e }
		if prepare {
			if _, err := engine.PreparePR(ctx, dir, env, req.Prepare, emit); err != nil {
				return prPlanMsg{err: err}
			}
		}
		plan, err := engine.PlanCreatePR(ctx, dir, env, req, emit)
		return prPlanMsg{plan: plan, err: err}
	}
	return m, tea.Batch(cmd, waitForPREvent(ch), tickSpinner())
}

// updatePR handles the messages of the Pull Request flow.
func (m model) updatePR(msg tea.Msg) (model, tea.Cmd) {
	if m.pr == nil {
		m.pr = &prFlow{}
	}
	f := m.pr
	switch msg := msg.(type) {
	case prEventMsg:
		f.stage, f.message = msg.Stage, msg.Message
		if msg.Stage == engine.StageAgent {
			if fields := strings.Fields(msg.Message); len(fields) > 0 {
				f.agent = fields[0]
			}
		}
		if msg.Quality != nil {
			m.applyQualityEvent(*msg.Quality)
		}
		if m.prEvents != nil {
			return m, waitForPREvent(m.prEvents)
		}
		return m, nil
	case prPlanMsg:
		if msg.err != nil {
			return m.withResult(genericFailure(failed(itemPR.Title, msg.err))), nil
		}
		f.plan = msg.plan
		m.screen, m.scrollOffset = screenPRPreview, 0
		return m, nil
	case prEditorMsg:
		if msg.err != nil {
			f.hint = msg.err.Error()
			return m, nil
		}
		return m, tea.ExecProcess(msg.cmd, func(err error) tea.Msg { return prEditedMsg{err: err} })
	case prEditedMsg:
		if msg.err != nil {
			f.hint = "editor: " + msg.err.Error()
			return m, nil
		}
		// The saved draft still belongs to HEAD, so planning reloads and
		// revalidates it.
		next, cmd := m.startPR(false)
		return next, cmd
	case prCreatedMsg:
		if msg.err != nil {
			return m.withResult(genericFailure(failed(itemPR.Title, msg.err))), nil
		}
		plan := f.plan
		return m.withResult(rendered(itemPR.Title, Success, func(p render.Printer) { render.PRCreate(p, plan, msg.rep) })), nil
	}
	return m, nil
}

// prPreviewKey handles the keys of the preview screen.
func (m model) prPreviewKey(k string) (model, tea.Cmd) {
	f := m.pr
	content, height := m.prPreviewContent()
	if m.scroll(k, content, height) {
		return m, nil
	}
	switch k {
	case "e":
		r := f.plan.Report
		if !m.info.Initialized || r.DraftPath == "" {
			f.hint = "Editing needs a saved draft (run: tija init)"
			return m, nil
		}
		f.hint = ""
		return m, editorCmd(m.env, r.Root, filepath.Join(r.Root, r.DraftPath))
	case "r":
		return m.startPR(true)
	case "enter":
		if len(f.plan.Report.Problems) > 0 {
			f.hint = "Fix the problems first (e edit · r regenerate)"
			return m, nil
		}
		f.hint = ""
		m.screen = screenConsent
	case "esc":
		return m.backToMenu()
	}
	return m, nil
}

// prConsentKey handles the consent screen of the Pull Request flow.
func (m model) prConsentKey(k string) (model, tea.Cmd) {
	switch k {
	case "y":
		m.screen = screenRunning
		dir, env, plan, req := m.dir, m.env, m.pr.plan, m.pr.req
		return m, func() tea.Msg {
			rep, err := engine.ApplyCreatePR(context.Background(), dir, env, plan, req)
			return prCreatedMsg{rep: rep, err: err}
		}
	case "n", "esc":
		m.screen = screenPRPreview
	}
	return m, nil
}

func (m model) prConsentView() string {
	return headingStyle.Render(m.pr.plan.Question()) + "\n\n" + sandStyle.Render("y/n")
}

// prWorkingView is the mascot state and body of the working screen.
func (m model) prWorkingView() (MascotState, string) {
	f := m.pr
	if f == nil {
		return Scanning, mutedStyle.Render("Preparing...")
	}
	switch f.stage {
	case engine.StageAgent:
		text := "Drafting..."
		if f.agent != "" {
			text = "Drafting with " + f.agent + "..."
		}
		return Thinking, sandStyle.Render(text)
	case engine.StageCheck:
		return Scanning, mutedStyle.Render("Running check...") + "\n\n" + m.qualityView()
	}
	msg := f.message
	if msg == "" {
		msg = "preparing"
	}
	return Scanning, mutedStyle.Render(capitalizeFirst(msg) + "...")
}

// prPreviewContent is the scrollable preview and the height it may use:
// the hint, when shown, sits under it.
func (m model) prPreviewContent() (string, int) {
	r := m.pr.plan.Report
	lines := []string{headingStyle.Render(r.Title), "", strings.TrimRight(r.Body, "\n"), ""}
	if len(r.Problems) == 0 {
		lines = append(lines, okStyle.Render("✓")+" title and sections pass pr-governance rules")
	}
	for _, p := range r.Problems {
		lines = append(lines, failStyle.Render("✗")+" "+p.Message)
	}
	for _, w := range r.Warnings {
		lines = append(lines, warnStyle.Render("!")+" "+w)
	}
	height := m.height
	if m.pr.hint != "" && height > 0 {
		height -= 2
	}
	return strings.Join(lines, "\n"), height
}

// prPreviewView is the mascot state, caption and body of the preview.
func (m model) prPreviewView() (MascotState, string, string) {
	content, height := m.prPreviewContent()
	body, _, _ := wrapAndSliceResult(content, m.width, height, m.scrollOffset)
	if m.pr.hint != "" {
		body += "\n\n" + mutedStyle.Render(m.pr.hint)
	}
	if len(m.pr.plan.Report.Problems) > 0 {
		return Warning, "", body
	}
	return Success, prReadyCaption, body
}

// prReadyCaption replaces "All tasks completed!" while the pull request is
// still to be created.
const prReadyCaption = "Draft ready."

// editorCmd resolves the editor git would use and returns it, ready to run
// on path, as a prEditorMsg.
func editorCmd(env sys.Env, root, path string) tea.Cmd {
	return func() tea.Msg {
		c, err := sys.EditorCommand(context.Background(), env, root, path)
		return prEditorMsg{cmd: c, err: err}
	}
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
