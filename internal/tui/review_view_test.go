package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/review"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

func TestMenuHasReview(t *testing.T) {
	for _, initialized := range []bool{false, true} {
		items := MenuItems(initialized, false, false, false)
		for i, item := range items {
			if item.Key == keyPR {
				if i+1 >= len(items) || items[i+1].Key != keyReview {
					t.Fatal(items)
				}
			}
		}
	}
}
func TestReviewModeSelection(t *testing.T) {
	m := newModel("repo", "test", sys.Env{})
	m, _ = m.startReview()
	for _, name := range []string{"Review", "Quality", "Deep"} {
		if !strings.Contains(m.reviewBody(), name) {
			t.Fatal(m.reviewBody())
		}
	}
	called := ""
	m.reviewRunner = func(_ context.Context, _ string, _ sys.Env, req engine.ReviewRequest, emit func(engine.ReviewEvent)) (engine.ReviewReport, error) {
		called = req.Mode
		emit(engine.ReviewEvent{Stage: engine.StageAgent, Pass: "quality", Chunk: 1, Chunks: 2})
		return engine.ReviewReport{}, nil
	}
	m, _ = m.reviewKey("down")
	m, _ = m.reviewKey("down")
	m, cmd := m.reviewKey("enter")
	if !m.review.working || m.mascot != Thinking {
		t.Fatal(m.review, m.mascot)
	}
	batch := cmd().(tea.BatchMsg)
	done := batch[0]()
	if called != "deep" {
		t.Fatal(called)
	}
	got, _ := m.Update(done)
	if got.(model).review.working {
		t.Fatal("still working")
	}
}
func TestReviewProgress(t *testing.T) {
	m := newModel("", "", sys.Env{})
	m, _ = m.startReview()
	m.review.working = true
	m.mascot = Thinking
	m.review.events = make(chan engine.ReviewEvent, 1)
	updated, _ := m.Update(reviewEventMsg(engine.ReviewEvent{Stage: engine.StageAgent, Pass: "quality", Chunk: 2, Chunks: 3, Message: "reviewing part"}))
	m = updated.(model)
	body := m.reviewBody()
	for _, want := range []string{"quality", "2/3", "reviewing part"} {
		if !strings.Contains(body, want) {
			t.Fatal(body)
		}
	}
	if m.mascot != Thinking {
		t.Fatal(m.mascot)
	}
}
func TestReviewResults(t *testing.T) {
	for _, tc := range []struct {
		exit   int
		err    error
		mascot MascotState
	}{{0, nil, Success}, {1, nil, Warning}, {2, nil, Error}, {0, errors.New("failed"), Error}} {
		t.Run(fmt.Sprint(tc.exit, tc.err), func(t *testing.T) {
			m := newModel("", "", sys.Env{})
			m.height = 12
			m, _ = m.startReview()
			rep := engine.ReviewReport{Result: review.Result{Exit: tc.exit, Mode: "deep", Findings: []review.Finding{{Severity: review.Low, File: "b.ts", Line: 3, Rule: "low"}, {Severity: review.High, File: "a.ts", Line: 2, Rule: "bug", Explanation: strings.Repeat("explanation\n", 20), Suggestion: "fix it", Evidence: "real code"}}}}
			updated, _ := m.Update(reviewDoneMsg{rep, tc.err})
			m = updated.(model)
			if m.mascot != tc.mascot {
				t.Fatal(m.mascot)
			}
			if m.review.report.Findings[0].Severity != review.High {
				t.Fatal("not sorted")
			}
			m, _ = m.reviewKey("enter")
			body := m.reviewBody()
			for _, want := range []string{"explanation", "fix it", "real code"} {
				if !strings.Contains(body, want) {
					t.Fatal(body)
				}
			}
			m, _ = m.reviewKey("down")
			if m.scrollOffset != 1 {
				t.Fatal(m.scrollOffset)
			}
			m, _ = m.reviewKey("esc")
			if m.review.detail {
				t.Fatal("still detail")
			}
			m, _ = m.reviewKey("q")
			if m.screen != screenDetecting || m.review != nil {
				t.Fatal("not returning to menu")
			}
		})
	}
}
func TestReviewSelectionScroll(t *testing.T) {
	m := newModel("", "", sys.Env{})
	m.height = 12
	m, _ = m.startReview()
	rep := engine.ReviewReport{}
	for i := 0; i < 25; i++ {
		rep.Findings = append(rep.Findings, review.Finding{Severity: review.High, File: fmt.Sprintf("file%02d.ts", i), Line: 1})
	}
	updated, _ := m.Update(reviewDoneMsg{report: rep})
	m = updated.(model)
	for i := 0; i < 15; i++ {
		m, _ = m.reviewKey("down")
	}
	if m.review.cursor != 15 || m.scrollOffset == 0 {
		t.Fatal(m.review.cursor, m.scrollOffset)
	}
	body, _, _ := wrapAndSliceResult(m.reviewBody(), m.width, m.height, m.scrollOffset)
	if !strings.Contains(body, "> file15.ts") {
		t.Fatal(body)
	}
}
