package workflows

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestClassifySteps(t *testing.T) {
	cases := []struct {
		run    string
		effect Effect
		unsafe bool
	}{
		{"status", EffectRead, false},
		{"doctor", EffectRead, false},
		{"review", EffectLocal, false},
		{"check", EffectLocal, false},
		{"test", EffectLocal, false},
		{"pr prepare", EffectLocal, false},
		{"lint", EffectLocal, false},
		{"lint --fix", EffectRepo, false},
		{"lint --fix=false", EffectLocal, false},
		{"format", EffectRepo, false},
		{"clean comments", EffectLocal, false},
		{"clean comments --write", EffectRepo, false},
		{"clean comments --write --unsafe", EffectRepo, true},
		{"pr create", EffectExternal, false},
		{"pr create --draft", EffectExternal, false},
	}
	for _, c := range cases {
		e, unsafe, err := Classify(Step{Run: c.run})
		if err != nil || e != c.effect || unsafe != c.unsafe {
			t.Errorf("Classify(%q) = %v, %v, %v; want %v, %v", c.run, e, unsafe, err, c.effect, c.unsafe)
		}
	}
	for _, bad := range []string{"pr create --edit", "rm -rf /", "run pre-pr"} {
		if _, _, err := Classify(Step{Run: bad}); err == nil {
			t.Errorf("Classify(%q) accepted", bad)
		}
	}
}

func TestRequirementsPrePR(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "assets", "defaults", "workflows", "pre-pr.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var w Workflow
	if err := yaml.Unmarshal(data, &w); err != nil {
		t.Fatal(err)
	}
	n, err := Requirements(w)
	if err != nil {
		t.Fatal(err)
	}
	if !n.Write || n.Unsafe || n.Publish {
		t.Errorf("pre-pr needs = %+v, want write only", n)
	}
	if len(n.Steps) != len(w.Steps) || n.Steps[0].Run != "clean comments --write" || n.Steps[0].Class != EffectRepo || n.Steps[0].Effect != "repo" {
		t.Errorf("steps = %+v", n.Steps)
	}
}

type fakeExec struct {
	calls [][]string
	exits map[string]int
}

func (f *fakeExec) run(_ context.Context, args []string, out, _ io.Writer) int {
	f.calls = append(f.calls, args)
	fmt.Fprintf(out, `{"cmd":%q}`, args[0])
	return f.exits[args[0]]
}

func TestRunMatchesOldBehaviour(t *testing.T) {
	w := Workflow{Name: "w", Steps: []Step{{Run: "status --json"}, {Run: "check", ContinueOnError: true}, {Run: "review"}, {Run: "doctor"}}}
	f := &fakeExec{exits: map[string]int{"check": 1, "review": 2}}
	var seen []int
	rep := Run(context.Background(), w, f.run, Options{JSON: true, Yes: true, OnStep: func(i int, _ StepResult) { seen = append(seen, i) }})

	if len(f.calls) != 3 {
		t.Fatalf("calls = %v, want stop after the failing review", f.calls)
	}
	for _, c := range f.calls {
		if n := strings.Count(strings.Join(c, " "), "--json"); n != 1 {
			t.Errorf("%v has --json %d times", c, n)
		}
		if !slices.Contains(c, "--yes") {
			t.Errorf("%v lacks --yes", c)
		}
	}
	if rep.Exit != 2 || len(rep.Steps) != 3 || !slices.Equal(seen, []int{0, 1, 2}) {
		t.Errorf("report = %+v, seen = %v", rep, seen)
	}
	if string(rep.Steps[0].Output) != `{"cmd":"status"}` {
		t.Errorf("output = %s", rep.Steps[0].Output)
	}
}

func TestRunPlainPrintsSteps(t *testing.T) {
	var out strings.Builder
	f := &fakeExec{}
	Run(context.Background(), Workflow{Steps: []Step{{Run: "status"}}}, f.run, Options{Out: &out})
	if !strings.Contains(out.String(), "> status") || slices.Contains(f.calls[0], "--json") {
		t.Errorf("out = %q, calls = %v", out.String(), f.calls)
	}
}
