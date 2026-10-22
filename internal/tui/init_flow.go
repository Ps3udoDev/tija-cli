package tui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/render"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

// initFlow walks the user through the choices and consent actions of an
// init plan before applying it.
type initFlow struct {
	plan     engine.InitPlan
	choice   int // index into plan.Choices
	cursor   int // selected option of the current choice
	answers  map[string]string
	consent  []engine.Action // actions that need consent
	action   int             // index into consent
	approved map[string]bool
}

// planMsg delivers the result of engine.PlanInit.
type planMsg struct {
	plan engine.InitPlan
	err  error
}

func planCmd(dir string, env sys.Env) tea.Cmd {
	return func() tea.Msg {
		plan, err := engine.PlanInit(context.Background(), dir, env)
		return planMsg{plan: plan, err: err}
	}
}

// startFlow begins answering the plan, or shows the planning error.
func (m model) startFlow(msg planMsg) (model, tea.Cmd) {
	if msg.err != nil {
		return m.withResult(failed(itemInit.Title, msg.err)), nil
	}
	f := &initFlow{plan: msg.plan, answers: map[string]string{}, approved: map[string]bool{}}
	for _, a := range msg.plan.Actions {
		if a.NeedsConsent {
			f.consent = append(f.consent, a)
		}
	}
	m.flow = f
	m.title = itemInit.Title
	return m.nextStep()
}

// nextStep moves to the pending choice, then the pending action, then
// applies the plan.
func (m model) nextStep() (model, tea.Cmd) {
	f := m.flow
	if f.choice < len(f.plan.Choices) {
		c := f.plan.Choices[f.choice]
		f.cursor = max(slices.Index(c.Options, c.Default), 0)
		m.screen = screenChoice
		return m, nil
	}
	if f.action < len(f.consent) {
		m.screen = screenConsent
		return m, nil
	}
	m.screen = screenRunning
	plan, answers, approved := f.plan, f.answers, f.approved
	return m, func() tea.Msg {
		res, err := engine.ApplyInit(context.Background(), plan, answers, approved)
		if err != nil {
			return failed(itemInit.Title, err)
		}
		return rendered(itemInit.Title, true, func(p render.Printer) { render.InitResult(p, res) })
	}
}

func (m model) withResult(r resultMsg) model {
	next, _ := m.Update(r)
	return next.(model)
}

func (m model) choiceKey(k string) (model, tea.Cmd) {
	f := m.flow
	c := f.plan.Choices[f.choice]
	switch k {
	case "up", "k":
		f.cursor = max(f.cursor-1, 0)
	case "down", "j":
		f.cursor = min(f.cursor+1, len(c.Options)-1)
	case "enter":
		f.answers[c.ID] = c.Options[f.cursor]
		f.choice++
		return m.nextStep()
	case "esc":
		return m.cancelFlow()
	}
	return m, nil
}

func (m model) consentKey(k string) (model, tea.Cmd) {
	f := m.flow
	a := f.consent[f.action]
	switch k {
	case "y", "n":
		f.approved[a.ID] = k == "y"
		f.action++
		return m.nextStep()
	case "esc":
		return m.cancelFlow()
	}
	return m, nil
}

// cancelFlow abandons init without applying anything.
func (m model) cancelFlow() (model, tea.Cmd) {
	m.flow = nil
	m.screen = screenMenu
	return m, nil
}

func (m model) choiceView() string {
	f := m.flow
	c := f.plan.Choices[f.choice]
	lines := []string{headingStyle.Render(c.Question), ""}
	for i, o := range c.Options {
		if i == f.cursor {
			lines = append(lines, selectedStyle.Render("> "+o))
		} else {
			lines = append(lines, "  "+o)
		}
	}
	return strings.Join(lines, "\n")
}

// maxDiffLines keeps the consent screen within the terminal.
const maxDiffLines = 20

func (m model) consentView() string {
	f := m.flow
	a := f.consent[f.action]
	msg := fmt.Sprintf("(%d/%d) %s %s", f.action+1, len(f.consent), a.Kind, a.Path)
	if a.Reason != "" {
		msg += " — " + a.Reason
	}
	lines := []string{headingStyle.Render(msg)}
	if a.Diff != "" {
		diff := strings.Split(strings.TrimRight(a.Diff, "\n"), "\n")
		limit := maxDiffLines
		if m.height > 0 {
			limit = max(m.height-16, 3)
		}
		if len(diff) > limit {
			diff = append(diff[:limit], fmt.Sprintf("… %d more lines", len(diff)-limit))
		}
		lines = append(lines, "")
		lines = append(lines, diff...)
	}
	lines = append(lines, "", sandStyle.Render("Apply? y/n"))
	return strings.Join(lines, "\n")
}
