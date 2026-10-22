package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Ps3udoDev/tija-cli/internal/detect"
	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
	"github.com/charmbracelet/x/ansi"
)

func titles(items []MenuItem) []string {
	var out []string
	for _, it := range items {
		out = append(out, it.Title)
	}
	return out
}

func TestMenuItemsPhase2(t *testing.T) {
	if got, want := titles(MenuItems(false)), []string{"Init", "Check", "Tests", "Doctor", "Status"}; !slices.Equal(got, want) {
		t.Errorf("MenuItems(false) = %v, want %v", got, want)
	}
	if got, want := titles(MenuItems(true)), []string{"Check", "Tests", "Doctor", "Status", "Init"}; !slices.Equal(got, want) {
		t.Errorf("MenuItems(true) = %v, want %v", got, want)
	}
	desc := map[string]string{
		"Check":  "Lint, format and tests",
		"Tests":  "Run the test suite",
		"Init":   "Detect the stack and set up .tija/",
		"Doctor": "Check tools, manifest and agents",
		"Status": "Branch, commits and working tree",
	}
	for _, it := range MenuItems(false) {
		if it.Description != desc[it.Title] {
			t.Errorf("%s description = %q, want %q", it.Title, it.Description, desc[it.Title])
		}
	}
}

func TestSpritesDistinctAndBounded(t *testing.T) {
	states := []MascotState{Idle, Scanning, Success, Warning, Error}
	seen := map[string]MascotState{}
	for _, s := range states {
		sp := Sprite(s)
		if sp == "" {
			t.Fatalf("Sprite(%s) is empty", s)
		}
		if prev, dup := seen[sp]; dup {
			t.Errorf("Sprite(%s) equals Sprite(%s)", s, prev)
		}
		seen[sp] = s
		lines := strings.Split(sp, "\n")
		if len(lines) > 8 {
			t.Errorf("Sprite(%s) has %d lines, want <= 8", s, len(lines))
		}
		for i, l := range lines {
			if w := lipgloss.Width(l); w > 16 {
				t.Errorf("Sprite(%s) line %d is %d cells wide, want <= 16", s, i, w)
			}
		}
	}
}

// fakeInfo is a detected, not yet initialized repository.
func fakeInfo() startupInfo {
	return startupInfo{
		Root:   "/projects/claims-api",
		InRepo: true,
		Detection: &detect.Detection{
			ProjectName: "ClaimsAPI",
			Framework:   ".NET 8",
			GitHost:     "github",
			TestCount:   74,
		},
	}
}

func menuModel(t *testing.T, width int) model {
	t.Helper()
	m := newModel("/projects/claims-api", "0.1.0", sys.Env{})
	m = step(t, m, tea.WindowSizeMsg{Width: width, Height: 30})
	return step(t, m, startupMsg{info: fakeInfo()})
}

// step feeds msg to m and returns the updated model.
func step(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	next, _ := m.Update(msg)
	mm, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", next)
	}
	return mm
}

func plainView(m model) string { return ansi.Strip(m.View().Content) }

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestNavigation(t *testing.T) {
	m := menuModel(t, 100)
	m, _ = m.handleKey("down")
	if m.cursor != 1 {
		t.Fatalf("after down cursor = %d, want 1", m.cursor)
	}
	m, _ = m.handleKey("up")
	m, _ = m.handleKey("up")
	if m.cursor != 0 {
		t.Fatalf("after up at 0 cursor = %d, want 0", m.cursor)
	}
	for i := 0; i < 10; i++ {
		m, _ = m.handleKey("j")
	}
	if m.cursor != 4 {
		t.Fatalf("after j past end cursor = %d, want 4", m.cursor)
	}
	m, _ = m.handleKey("k")
	if m.cursor != 3 {
		t.Fatalf("after k cursor = %d, want 3", m.cursor)
	}
	if _, cmd := m.handleKey("q"); !isQuit(cmd) {
		t.Fatal("\"q\" did not quit")
	}
	if _, cmd := m.handleKey("ctrl+c"); !isQuit(cmd) {
		t.Fatal("\"ctrl+c\" did not quit")
	}
}

func TestStartupViewContents(t *testing.T) {
	for _, width := range []int{120, 60} {
		v := plainView(menuModel(t, width))
		for _, want := range []string{
			"TIJA", "Repository engineering for humans & agents", "What do you want to do?",
			"> Init", "Detect the stack and set up .tija/",
			"✓ ClaimsAPI detected", "✓ .NET 8", "✓ GitHub", "✓ 74 tests",
			"v0.1.0", "( o.o )",
		} {
			if !strings.Contains(v, want) {
				t.Errorf("width %d: view lacks %q:\n%s", width, want, v)
			}
		}
		for i, l := range strings.Split(v, "\n") {
			if w := lipgloss.Width(l); w > width {
				t.Errorf("width %d: line %d is %d cells wide", width, i, w)
			}
		}
	}
}

func TestStartupViewStacksWhenNarrow(t *testing.T) {
	v := plainView(menuModel(t, 60))
	question := strings.Index(v, "What do you want to do?")
	if question < 0 || !strings.Contains(v, "74 tests") {
		t.Fatalf("missing checklist or question:\n%s", v)
	}
	if lineOf(v, question) <= lineOf(v, strings.Index(v, "74 tests")) {
		t.Errorf("narrow view does not stack the menu under the left column:\n%s", v)
	}
	wide := plainView(menuModel(t, 120))
	if lineOf(wide, strings.Index(wide, "What do you want to do?")) > lineOf(wide, strings.Index(wide, "74 tests")) {
		t.Errorf("wide view does not place the menu beside the left column:\n%s", wide)
	}
}

func lineOf(s string, idx int) int { return strings.Count(s[:idx], "\n") }

func TestStartupViewNoTests(t *testing.T) {
	m := newModel("/x", "dev", sys.Env{})
	info := fakeInfo()
	info.Detection.TestCount = 0
	m = step(t, m, startupMsg{info: info})
	if v := plainView(m); !strings.Contains(v, "! no tests detected") {
		t.Errorf("view lacks no-tests warning:\n%s", v)
	}
}

func TestStartupViewNotARepo(t *testing.T) {
	m := newModel("/x", "dev", sys.Env{})
	m = step(t, m, startupMsg{info: startupInfo{Root: "/x"}})
	v := plainView(m)
	if !strings.Contains(v, "! not a git repository") {
		t.Errorf("view lacks not-a-repo warning:\n%s", v)
	}
	if strings.Contains(v, "detected") {
		t.Errorf("view shows a detection checklist outside a repo:\n%s", v)
	}
	if !strings.Contains(v, "> Init") {
		t.Errorf("menu should start at Init:\n%s", v)
	}
}

func TestDetectingShowsScanningMascot(t *testing.T) {
	m := newModel("/x", "dev", sys.Env{})
	if !hasSprite(m.View().Content, Scanning) {
		t.Error("detecting view does not show the scanning mascot")
	}
	m = step(t, m, startupMsg{info: fakeInfo()})
	if !hasSprite(m.View().Content, Idle) {
		t.Error("menu view does not show the idle mascot")
	}
}

func TestLoadStartup(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	env := sys.Default()
	info := loadStartup(repo, env)
	if !info.InRepo || info.Detection == nil {
		t.Fatalf("info = %+v, want a detected repo", info)
	}
	if info.Initialized {
		t.Error("Initialized = true before init")
	}
	p := filepath.Join(repo, filepath.FromSlash(manifest.Path))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if info := loadStartup(repo, env); !info.Initialized {
		t.Error("Initialized = false with a manifest")
	}

	if info := loadStartup(t.TempDir(), env); info.InRepo || info.Detection != nil {
		t.Errorf("outside a repo info = %+v", info)
	}
}

func fixture(name string) string {
	p, err := filepath.Abs(filepath.Join("..", "detect", "testdata", name))
	if err != nil {
		panic(err)
	}
	return p
}

// drive runs cmd and feeds its message back to m, as the runtime would.
func drive(t *testing.T, m model, cmd tea.Cmd) model {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command")
	}
	return step(t, m, cmd())
}

func TestDoctorResultAndBack(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	m := newModel(repo, "dev", sys.Default())
	m = step(t, m, startupMsg{info: loadStartup(repo, sys.Default())})
	for i, it := range m.items {
		if it.Key == keyDoctor {
			m.cursor = i
			break
		}
	}
	m, cmd := m.handleKey("enter")
	if m.screen != screenRunning {
		t.Fatalf("screen = %v, want running", m.screen)
	}
	m = drive(t, m, cmd)
	if m.screen != screenResult {
		t.Fatalf("screen = %v, want result", m.screen)
	}
	v := plainView(m)
	if !strings.Contains(v, "Git installed") {
		t.Errorf("result lacks doctor output:\n%s", v)
	}
	m, cmd = m.handleKey("esc")
	if m.screen != screenDetecting {
		t.Fatalf("after esc screen = %v, want detecting", m.screen)
	}
	m = drive(t, m, cmd)
	if m.screen != screenMenu {
		t.Fatalf("screen = %v, want menu", m.screen)
	}
}

func TestStatusOutsideRepoShowsError(t *testing.T) {
	dir := t.TempDir()
	m := newModel(dir, "dev", sys.Default())
	m = step(t, m, startupMsg{info: loadStartup(dir, sys.Default())})
	m, _ = m.handleKey("down")
	for i, it := range m.items {
		if it.Key == keyStatus {
			m.cursor = i
			break
		}
	}
	m, cmd := m.handleKey("enter")
	m = drive(t, m, cmd)
	if m.ok {
		t.Error("status outside a repo reported success")
	}
	v := m.View().Content
	if !strings.Contains(ansi.Strip(v), "not a git repository") {
		t.Errorf("result lacks the error:\n%s", ansi.Strip(v))
	}
	if !hasSprite(v, Error) {
		t.Error("error result does not show the error mascot")
	}
}

func TestInitFlowConsent(t *testing.T) {
	repo := testutil.NewRepo(t, fixture("react-vite"))
	gitignore := filepath.Join(repo, ".gitignore")
	before := []byte("node_modules/\n")
	if err := os.WriteFile(gitignore, before, 0o644); err != nil {
		t.Fatal(err)
	}
	m := newModel(repo, "dev", sys.Default())
	m = step(t, m, startupMsg{info: loadStartup(repo, sys.Default())})
	m, cmd := m.handleKey("enter") // Init
	m = drive(t, m, cmd)

	// Answer every choice with its default.
	for m.screen == screenChoice {
		m, cmd = m.handleKey("enter")
	}
	if m.screen != screenConsent {
		t.Fatalf("screen = %v, want consent", m.screen)
	}
	if v := plainView(m); !strings.Contains(v, ".gitignore") || !strings.Contains(v, "y/n") {
		t.Errorf("consent view lacks the action or prompt:\n%s", v)
	}
	// Reject every action that needs consent.
	for m.screen == screenConsent {
		m, cmd = m.handleKey("n")
	}
	if m.screen != screenRunning {
		t.Fatalf("screen = %v, want running", m.screen)
	}
	m = drive(t, m, cmd)
	if m.screen != screenResult || !m.ok {
		t.Fatalf("screen = %v ok = %v, want successful result", m.screen, m.ok)
	}
	if v := plainView(m); !strings.Contains(v, "Pending") {
		t.Errorf("result lacks rejected actions as pending:\n%s", v)
	}
	after, _ := os.ReadFile(gitignore)
	if string(after) != string(before) {
		t.Error(".gitignore changed although its action was rejected")
	}
	if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(manifest.Path))); err != nil {
		t.Errorf("manifest not created: %v", err)
	}

	// Back to the menu: the repository is now initialized.
	m, cmd = m.handleKey("esc")
	m = drive(t, m, cmd)
	if got := titles(m.items); !slices.Equal(got, titles(MenuItems(true))) {
		t.Errorf("menu after init = %v, want %v", got, titles(MenuItems(true)))
	}
}

func TestChoiceNavigation(t *testing.T) {
	m := newModel("/x", "dev", sys.Env{})
	m = step(t, m, startupMsg{info: fakeInfo()})
	plan := engine.InitPlan{Choices: []engine.Choice{{ID: "target", Question: "Target branch?", Options: []string{"main", "develop"}, Default: "develop"}}}
	m = step(t, m, planMsg{plan: plan})
	if m.screen != screenChoice {
		t.Fatalf("screen = %v, want choice", m.screen)
	}
	if v := plainView(m); !strings.Contains(v, "> develop") {
		t.Errorf("choice view does not preselect the default:\n%s", v)
	}
	m, _ = m.handleKey("up")
	if m.flow.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", m.flow.cursor)
	}
	m, _ = m.handleKey("esc")
	if m.screen != screenMenu {
		t.Errorf("esc in a choice: screen = %v, want menu", m.screen)
	}
}

// hasSprite reports whether every line of the state's sprite appears in view.
func hasSprite(view string, s MascotState) bool {
	for _, l := range strings.Split(Sprite(s), "\n") {
		if !strings.Contains(view, l) {
			return false
		}
	}
	return true
}

func TestDisplayVersion(t *testing.T) {
	for in, want := range map[string]string{"0.1.0": "v0.1.0", "dev": "dev", "": ""} {
		if got := displayVersion(in); got != want {
			t.Errorf("displayVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStartupViewTestFiles(t *testing.T) {
	m := newModel("/x", "dev", sys.Env{})
	info := fakeInfo()
	info.Detection.TestCount, info.Detection.TestFiles = 0, 5
	m = step(t, m, startupMsg{info: info})
	if v := plainView(m); !strings.Contains(v, "5 test files") || strings.Contains(v, "no tests detected") {
		t.Errorf("view should list 5 test files:\n%s", v)
	}
}


func TestQualityProgressView(t *testing.T) {
	m := newModel("/projects/claims-api", "0.1.0", sys.Env{})
	m = step(t, m, tea.WindowSizeMsg{Width: 80, Height: 30})
	m.screen = screenQuality
	m.title = "Check"

	ev0Start := engine.QualityEvent{
		Type:  engine.EventStart,
		Index: 0,
		Total: 2,
		Step: quality.StepResult{
			Kinds:   []quality.Kind{quality.Build},
			Command: "build",
		},
	}
	m = step(t, m, qualityEventMsg(ev0Start))

	ev0Done := engine.QualityEvent{
		Type:  engine.EventDone,
		Index: 0,
		Total: 2,
		Step: quality.StepResult{
			Kinds:      []quality.Kind{quality.Build},
			Command:    "build",
			Status:     quality.StatusPassed,
			DurationMs: 1200,
		},
	}
	m = step(t, m, qualityEventMsg(ev0Done))

	ev1Start := engine.QualityEvent{
		Type:  engine.EventStart,
		Index: 1,
		Total: 2,
		Step: quality.StepResult{
			Kinds:   []quality.Kind{quality.Tests},
			Command: "tests",
		},
	}
	m = step(t, m, qualityEventMsg(ev1Start))

	v := ansi.Strip(m.View().Content)
	if !strings.Contains(v, "✓ build") {
		t.Errorf("view lacks '✓ build':\n%s", v)
	}
	hasSpinner := false
	for _, f := range spinnerFrames {
		if strings.Contains(v, f) {
			hasSpinner = true
			break
		}
	}
	if !hasSpinner {
		t.Errorf("view lacks spinner frame for running step:\n%s", v)
	}
}

func TestQualityResultWarningMascot(t *testing.T) {
	m := newModel("/projects/claims-api", "0.1.0", sys.Env{})
	m = step(t, m, tea.WindowSizeMsg{Width: 80, Height: 30})
	m.title = "Check"
	m = step(t, m, resultMsg{
		title:  "Check",
		output: "Check\n✗ lint biome\n",
		mascot: Warning,
		ok:     false,
	})
	v := m.View().Content
	if !hasSprite(v, Warning) {
		t.Errorf("view does not contain Sprite(Warning):\n%s", v)
	}
}

func TestResultScroll(t *testing.T) {
	m := newModel("/projects/claims-api", "0.1.0", sys.Env{})
	m = step(t, m, tea.WindowSizeMsg{Width: 80, Height: 30})
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, strings.Repeat("line ", 1)+string(rune('0'+i/10))+string(rune('0'+i%10)))
	}
	m.title = "Check"
	m = step(t, m, resultMsg{
		title:  "Check",
		output: strings.Join(lines, "\n"),
		mascot: Success,
		ok:     true,
	})

	v := m.View().Content
	if strings.Contains(v, "line 99") {
		t.Errorf("initial view contains line 99:\n%s", v)
	}
	if !strings.Contains(v, "line 00") {
		t.Errorf("initial view lacks line 00:\n%s", v)
	}

	m, _ = m.handleKey("end")
	vEnd := m.View().Content
	if !strings.Contains(vEnd, "line 99") {
		t.Errorf("view after end lacks line 99:\n%s", vEnd)
	}
	if strings.Contains(vEnd, "line 00") {
		t.Errorf("view after end still contains line 00:\n%s", vEnd)
	}
}

func TestResultWraps(t *testing.T) {
	m := newModel("/projects/claims-api", "0.1.0", sys.Env{})
	m = step(t, m, tea.WindowSizeMsg{Width: 80, Height: 30})
	m.title = "Check"
	m = step(t, m, resultMsg{
		title:  "Check",
		output: strings.Repeat("x", 200),
		mascot: Success,
		ok:     true,
	})
	v := ansi.Strip(m.View().Content)
	for i, line := range strings.Split(v, "\n") {
		if w := lipgloss.Width(line); w > 80 {
			t.Errorf("line %d width = %d > 80: %q", i, w, line)
		}
	}
}

func TestStartupShowsFindRootError(t *testing.T) {
	info := startupInfo{
		Root:   "/projects/claims-api",
		InRepo: false,
		Err:    "fatal: detected dubious ownership in repository",
	}
	m := newModel("/projects/claims-api", "0.1.0", sys.Env{})
	m = step(t, m, startupMsg{info: info})
	v := m.View().Content
	if !strings.Contains(v, "detected dubious ownership") {
		t.Errorf("startup view lacks error message:\n%s", v)
	}
}

