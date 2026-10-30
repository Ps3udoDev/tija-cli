package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/Ps3udoDev/tija-cli/internal/workflows"
)

type WorkflowStep struct {
	Run  string
	Exit int
}

type WorkflowRunner func(name string, onStep func(i int, s WorkflowStep)) (exit int)

type workflowStepMsg struct {
	index int
	step  WorkflowStep
}

type workflowDoneMsg struct {
	exit int
}

func waitForWorkflowStep(ch <-chan workflowStepMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

type workflowState int

const (
	workflowSelecting workflowState = iota
	workflowRunningState
	workflowDone
)

type workflowFlow struct {
	state     workflowState
	workflows []workflows.Workflow
	cursor    int
	selected  *workflows.Workflow

	steps  []WorkflowStep
	exit   int
	events chan workflowStepMsg
}

func (m model) startWorkflow() (model, tea.Cmd) {
	m.screen = screenWorkflow
	m.title = "Workflows"
	m.mascot = Idle
	m.caption = ""

	wfs, _ := workflows.Load(m.info.Root)
	var valid []workflows.Workflow
	for _, w := range wfs {
		if len(workflows.Validate(w)) == 0 {
			valid = append(valid, w)
		}
	}

	if len(valid) == 0 {
		m.workflow = &workflowFlow{state: workflowDone}
		m.mascot = Warning
		return m, nil
	}

	if len(valid) == 1 {
		m.workflow = &workflowFlow{
			workflows: valid,
			cursor:    0,
		}
		return m.runSelectedWorkflow(valid[0])
	}

	m.workflow = &workflowFlow{
		state:     workflowSelecting,
		workflows: valid,
		cursor:    0,
	}
	return m, nil
}

func (m model) runSelectedWorkflow(wf workflows.Workflow) (model, tea.Cmd) {
	if m.workflow == nil {
		m.workflow = &workflowFlow{}
	}
	m.workflow.selected = &wf
	m.workflow.state = workflowRunningState
	m.workflow.steps = make([]WorkflowStep, len(wf.Steps))
	for i, st := range wf.Steps {
		m.workflow.steps[i] = WorkflowStep{Run: st.Run, Exit: -1}
	}
	m.mascot = Scanning
	m.caption = ""

	stepCh := make(chan workflowStepMsg, 32)
	m.workflow.events = stepCh

	runner := m.workflowRunner
	if runner == nil {
		runner = func(name string, onStep func(i int, s WorkflowStep)) int {
			return 0
		}
	}

	cmd := func() tea.Msg {
		exit := runner(wf.Name, func(i int, s WorkflowStep) {
			stepCh <- workflowStepMsg{index: i, step: s}
		})
		close(stepCh)
		return workflowDoneMsg{exit: exit}
	}

	return m, tea.Batch(cmd, waitForWorkflowStep(stepCh))
}

func (m model) workflowKey(k string) (model, tea.Cmd) {
	if m.workflow == nil {
		return m.backToMenu()
	}
	if m.workflow.state == workflowSelecting {
		switch k {
		case "up", "k":
			m.workflow.cursor = max(0, m.workflow.cursor-1)
		case "down", "j":
			m.workflow.cursor = min(len(m.workflow.workflows)-1, m.workflow.cursor+1)
		case "enter":
			if len(m.workflow.workflows) > 0 {
				return m.runSelectedWorkflow(m.workflow.workflows[m.workflow.cursor])
			}
		case "esc", "q":
			return m.backToMenu()
		}
		return m, nil
	}
	if m.workflow.state == workflowDone {
		switch k {
		case "esc", "q":
			return m.backToMenu()
		}
	}
	return m, nil
}

func (m model) workflowBody() string {
	if m.workflow == nil {
		return ""
	}
	if m.workflow.state == workflowSelecting {
		var lines []string
		lines = append(lines, headingStyle.Render("Select a workflow:")+"\n")
		for i, wf := range m.workflow.workflows {
			prefix := "  "
			title := itemTitleStyle.Render(wf.Name)
			if i == m.workflow.cursor {
				prefix = selArrowStyle.Render("> ")
				title = selTitleStyle.Render(wf.Name)
			}
			line := prefix + title
			if wf.Description != "" {
				line += "  " + mutedStyle.Render(wf.Description)
			}
			lines = append(lines, line)
		}
		return strings.Join(lines, "\n")
	}

	var lines []string
	if m.workflow.selected != nil {
		lines = append(lines, headingStyle.Render(m.workflow.selected.Name)+"\n")
	}
	if len(m.workflow.steps) == 0 {
		lines = append(lines, mutedStyle.Render("No steps executed"))
	}
	for _, step := range m.workflow.steps {
		var sym string
		switch {
		case step.Exit == 0:
			sym = okStyle.Render("✓")
		case step.Exit > 0:
			sym = failStyle.Render("✗")
		default:
			sym = mutedStyle.Render("·")
		}
		lines = append(lines, sym+" "+step.Run)
	}
	return strings.Join(lines, "\n")
}
