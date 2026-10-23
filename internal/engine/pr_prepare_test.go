package engine

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/prrules"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

const validAgentDraft = `Here is the draft:

<<<TITLE
feat(auth): enmascarar contraseñas en los logs
TITLE>>>
<<<BODY
## Resumen

Se enmascaran las contraseñas antes de escribirlas en los logs.

## Objetivo

Evitar que las contraseñas aparezcan en texto plano en los logs.

## Cambios realizados

- Se agrega la función mask en src/auth/password.ts.

## Pruebas

- No se ejecutaron verificaciones automáticas para este commit.
BODY>>>
`

// noTestsDraft lacks the required ## Pruebas section.
const noTestsDraft = `<<<TITLE
feat(auth): enmascarar contraseñas en los logs
TITLE>>>
<<<BODY
## Resumen

Se enmascaran las contraseñas antes de escribirlas en los logs.

## Objetivo

Evitar que las contraseñas aparezcan en texto plano en los logs.

## Cambios realizados

- Se agrega la función mask en src/auth/password.ts.
BODY>>>
`

type agentCall struct {
	stdin, name string
	args        []string
}

type agentReply struct {
	out  string
	code int
}

// fakeAgentEnv finds a fake "claude" on PATH and answers each RunInput call
// with the next reply, recording the prompts it received.
func fakeAgentEnv(replies ...agentReply) (sys.Env, *[]agentCall) {
	calls := &[]agentCall{}
	env := fakeEnv()
	look := env.LookPath
	env.LookPath = func(name string) (string, error) {
		if name == "claude" {
			return "/fake/bin/claude", nil
		}
		return look(name)
	}
	env.RunInput = func(ctx context.Context, dir, stdin, name string, args ...string) (string, string, int, error) {
		i := len(*calls)
		*calls = append(*calls, agentCall{stdin: stdin, name: name, args: args})
		if i >= len(replies) {
			return "", "unexpected call", 1, nil
		}
		r := replies[i]
		if r.code != 0 {
			return "", "boom", r.code, nil
		}
		return r.out, "", 0, nil
	}
	return env, calls
}

// prRepo returns an initialized react-vite repo, with init committed, on a
// feature branch with one commit.
func prRepo(t *testing.T) string {
	t.Helper()
	repo := initRepo(t, "react-vite")
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-m", "chore: init tija")
	testutil.Git(t, repo, "checkout", "-b", "feat/auth-mask")
	commitFile(t, repo, "src/auth/password.ts", "export const mask = (s: string) => '*'.repeat(s.length)\n", "feat(auth): mask passwords in logs")
	return repo
}

func prepare(t *testing.T, repo string, env sys.Env, req PRRequest) (PRReport, []PREvent) {
	t.Helper()
	var events []PREvent
	r, err := PreparePR(context.Background(), repo, env, req, func(e PREvent) { events = append(events, e) })
	if err != nil {
		t.Fatal(err)
	}
	return r, events
}

func hasWarning(r PRReport, sub string) bool {
	for _, w := range r.Warnings {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
}

func TestPreparePRWithAgent(t *testing.T) {
	repo := prRepo(t)
	env, calls := fakeAgentEnv(agentReply{out: validAgentDraft})

	r, _ := prepare(t, repo, env, PRRequest{Agent: "claude", NoCheck: true})
	if r.Source != "claude" || r.Attempts != 1 || len(r.Problems) != 0 {
		t.Fatalf("source=%q attempts=%d problems=%+v", r.Source, r.Attempts, r.Problems)
	}
	if r.ExitCode() != 0 {
		t.Errorf("ExitCode = %d", r.ExitCode())
	}
	if r.Title != "feat(auth): enmascarar contraseñas en los logs" || !strings.HasPrefix(r.Body, "## Resumen") {
		t.Errorf("title=%q body=%q", r.Title, r.Body)
	}
	if len(*calls) != 1 || (*calls)[0].name != "/fake/bin/claude" || !strings.Contains((*calls)[0].stdin, "<<<TITLE") {
		t.Errorf("calls = %+v", *calls)
	}
	if r.DraftPath != DraftPath {
		t.Errorf("DraftPath = %q", r.DraftPath)
	}
	d, meta, err := LoadDraft(repo)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != r.Title || meta.Source != "claude" || meta.Branch != "feat/auth-mask" || meta.Head == "" {
		t.Errorf("draft = %+v meta = %+v", d, meta)
	}
	if r.Branch != "feat/auth-mask" || r.Target != "main" || r.Commits != 1 || r.Files != 1 {
		t.Errorf("report = %+v", r)
	}
}

func TestPreparePRRetriesThenReports(t *testing.T) {
	repo := prRepo(t)
	env, calls := fakeAgentEnv(agentReply{out: noTestsDraft}, agentReply{out: noTestsDraft})

	r, _ := prepare(t, repo, env, PRRequest{Agent: "claude", NoCheck: true})
	if r.Attempts != 2 || r.Source != "claude" {
		t.Fatalf("attempts=%d source=%q", r.Attempts, r.Source)
	}
	if len(*calls) != 2 {
		t.Fatalf("calls = %d", len(*calls))
	}
	if strings.Contains((*calls)[0].stdin, "missing section ## Pruebas") {
		t.Error("first prompt must not carry corrections")
	}
	if !strings.Contains((*calls)[1].stdin, "missing section ## Pruebas") {
		t.Errorf("second prompt lacks the problem:\n%s", (*calls)[1].stdin)
	}
	if r.ExitCode() != 1 || len(r.Problems) == 0 {
		t.Errorf("ExitCode = %d problems = %+v", r.ExitCode(), r.Problems)
	}
	d, meta, err := LoadDraft(repo)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != r.Title || len(meta.Problems) == 0 {
		t.Errorf("saved draft = %+v meta = %+v", d, meta)
	}
}

func TestPreparePRRetrySucceeds(t *testing.T) {
	repo := prRepo(t)
	env, _ := fakeAgentEnv(agentReply{out: noTestsDraft}, agentReply{out: validAgentDraft})

	r, _ := prepare(t, repo, env, PRRequest{Agent: "claude", NoCheck: true})
	if r.Attempts != 2 || r.ExitCode() != 0 || len(r.Problems) != 0 || r.Source != "claude" {
		t.Errorf("attempts=%d exit=%d problems=%+v source=%q", r.Attempts, r.ExitCode(), r.Problems, r.Source)
	}
}

func TestPreparePRAgentFailureFallsBack(t *testing.T) {
	repo := prRepo(t)
	env, _ := fakeAgentEnv(agentReply{code: 1})

	r, _ := prepare(t, repo, env, PRRequest{Agent: "claude", NoCheck: true})
	if r.Source != "deterministic" {
		t.Errorf("Source = %q", r.Source)
	}
	if !hasWarning(r, "agent claude failed") || !hasWarning(r, "using deterministic draft") {
		t.Errorf("warnings = %v", r.Warnings)
	}
	if r.ExitCode() != 0 {
		t.Errorf("ExitCode = %d problems = %+v", r.ExitCode(), r.Problems)
	}
}

func TestPreparePRUnparseableFallsBack(t *testing.T) {
	repo := prRepo(t)
	env, _ := fakeAgentEnv(agentReply{out: "I cannot help with that."})

	r, _ := prepare(t, repo, env, PRRequest{Agent: "claude", NoCheck: true})
	if r.Source != "deterministic" || !hasWarning(r, "agent claude failed") {
		t.Errorf("source=%q warnings=%v", r.Source, r.Warnings)
	}
}

func TestPreparePRNoAgent(t *testing.T) {
	repo := prRepo(t)
	env, calls := fakeAgentEnv()

	r, events := prepare(t, repo, env, PRRequest{Agent: "none", NoCheck: true})
	if r.Source != "deterministic" || r.Attempts != 0 {
		t.Errorf("source=%q attempts=%d", r.Source, r.Attempts)
	}
	if len(*calls) != 0 {
		t.Errorf("RunInput called %d times", len(*calls))
	}
	if hasWarning(r, "no agent available") {
		t.Errorf("explicit none must not warn: %v", r.Warnings)
	}
	if len(events) == 0 || events[len(events)-1].Stage != "done" {
		t.Errorf("events = %+v", events)
	}
}

func TestPreparePRAgentUnavailable(t *testing.T) {
	repo := prRepo(t)
	env, calls := fakeAgentEnv()

	r, _ := prepare(t, repo, env, PRRequest{Agent: "codex", NoCheck: true})
	if r.Source != "deterministic" || len(*calls) != 0 || !hasWarning(r, "no agent available — using deterministic draft") {
		t.Errorf("source=%q calls=%d warnings=%v", r.Source, len(*calls), r.Warnings)
	}
}

func TestPreparePRRunsCheckWhenStale(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is not installed")
	}
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm is not installed")
	}
	repo := testutil.NewRepo(t, "")
	writeTestFile(t, repo, "package.json", `{"name":"x","private":true,"scripts":{"test":"node -e \"\""}}`+"\n")
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-m", "chore: add package.json")
	p := plan(t, repo)
	apply(t, p, nil, approveAll(p))
	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	m.Quality = map[string]manifest.QualityEntry{"tests": {Provider: "script", Script: "test"}}
	if err := manifest.Save(repo, m); err != nil {
		t.Fatal(err)
	}
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-m", "chore: init tija")
	testutil.Git(t, repo, "checkout", "-b", "feat/thing")
	commitFile(t, repo, "src/thing.js", "export const thing = 1\n", "feat(thing): add thing")
	head := testutil.Git(t, repo, "rev-parse", "HEAD")

	env := sys.Default()
	r, events := prepare(t, repo, env, PRRequest{Agent: "none"})
	sawCheck := false
	for _, e := range events {
		if e.Stage == "check" && e.Quality != nil {
			sawCheck = true
		}
	}
	if !sawCheck {
		t.Errorf("no check events in %+v", events)
	}
	if len(r.Tests) != 1 || r.Tests[0].Status != quality.StatusPassed || r.Tests[0].Commit != head {
		t.Fatalf("tests = %+v (head %s)", r.Tests, head)
	}
	if !strings.Contains(r.Body, "npm run test") {
		t.Errorf("body does not cite the check:\n%s", r.Body)
	}
}

func TestPreparePRNoCheckSkips(t *testing.T) {
	repo := prRepo(t)
	env, _ := fakeAgentEnv()

	r, events := prepare(t, repo, env, PRRequest{Agent: "none", NoCheck: true})
	for _, e := range events {
		if e.Stage == "check" {
			t.Errorf("unexpected check event %+v", e)
		}
	}
	if !strings.Contains(r.Body, "No se ejecutaron verificaciones") {
		t.Errorf("body:\n%s", r.Body)
	}
}

func TestPreparePRUninitialized(t *testing.T) {
	repo := testutil.NewRepo(t, "", testutil.WithBranch("feat/thing"))
	commitFile(t, repo, "src/thing.txt", "thing\n", "feat(thing): add thing")
	env, _ := fakeAgentEnv()

	r, _ := prepare(t, repo, env, PRRequest{Agent: "none"})
	if _, err := os.Stat(filepath.Join(repo, ".tija")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf(".tija must not exist: %v", err)
	}
	if r.DraftPath != "" {
		t.Errorf("DraftPath = %q", r.DraftPath)
	}
	if !hasWarning(r, "not initialized — draft is not saved (run: tija init)") {
		t.Errorf("warnings = %v", r.Warnings)
	}
}

func TestPreparePRInvalidManifest(t *testing.T) {
	repo := prRepo(t)
	writeTestFile(t, repo, manifest.Path, "version: 1\nagents:\n  default: bogus\n")
	env, _ := fakeAgentEnv()

	r, _ := prepare(t, repo, env, PRRequest{Agent: "none", NoCheck: true})
	if !hasWarning(r, "manifest invalid — using defaults; draft is not saved") || hasWarning(r, "not initialized") {
		t.Errorf("warnings = %v", r.Warnings)
	}
	if r.DraftPath != "" {
		t.Errorf("DraftPath = %q", r.DraftPath)
	}
	if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(DraftPath))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("draft must not be saved: %v", err)
	}
}

func TestPreparePRWarnsNonconformingAndDirty(t *testing.T) {
	repo := initRepo(t, "react-vite")
	testutil.Git(t, repo, "add", "-A")
	testutil.Git(t, repo, "commit", "-m", "chore: init tija")
	testutil.Git(t, repo, "checkout", "-b", "changes")
	commitFile(t, repo, "src/x.ts", "export const x = 1\n", "feat(x): add x")
	writeTestFile(t, repo, "src/y.ts", "export const y = 1\n")
	env, _ := fakeAgentEnv()

	r, _ := prepare(t, repo, env, PRRequest{Agent: "none", NoCheck: true})
	if !hasWarning(r, "branch changes does not follow the convention (suggested: ") {
		t.Errorf("warnings = %v", r.Warnings)
	}
	if !hasWarning(r, "uncommitted changes are not part of the pull request") {
		t.Errorf("warnings = %v", r.Warnings)
	}
}

func TestPRPrompt(t *testing.T) {
	repo := prRepo(t)
	env, calls := fakeAgentEnv()
	p, err := PRPrompt(context.Background(), repo, env)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p, "<<<TITLE") || !strings.Contains(p, "feat/auth-mask") || len(*calls) != 0 {
		t.Errorf("prompt = %q calls = %d", p, len(*calls))
	}
}

func TestPreparePRNotARepo(t *testing.T) {
	_, err := PreparePR(context.Background(), t.TempDir(), fakeEnv(), PRRequest{}, nil)
	if !errors.Is(err, ErrNotARepo) {
		t.Errorf("err = %v", err)
	}
}

// claimingDraft is an agent draft whose ## Pruebas claims tests nobody ran.
var claimingDraft = strings.Replace(validAgentDraft,
	"- No se ejecutaron verificaciones automáticas para este commit.",
	"- Todas las pruebas pasan correctamente en CI.", 1)

func pruebas(t *testing.T, body string) string {
	t.Helper()
	content, ok := prrules.SectionContent(body, "Pruebas")
	if !ok {
		t.Fatalf("body lacks ## Pruebas:\n%s", body)
	}
	return content
}

func TestPreparePRAgentCannotClaimTests(t *testing.T) {
	repo := prRepo(t)
	env, _ := fakeAgentEnv(agentReply{out: claimingDraft})

	r, _ := prepare(t, repo, env, PRRequest{Agent: "claude", NoCheck: true})
	got := pruebas(t, r.Body)
	if strings.Contains(got, "Todas las pruebas pasan") || got != "No se ejecutaron verificaciones automáticas para este commit." {
		t.Errorf("Pruebas = %q", got)
	}
	if !strings.Contains(r.Body, "## Cambios realizados") || r.Source != "claude" || len(r.Problems) != 0 {
		t.Errorf("source=%q problems=%+v body:\n%s", r.Source, r.Problems, r.Body)
	}
}

func TestPreparePRAgentPruebasCitesFreshResult(t *testing.T) {
	repo := prRepo(t)
	head := testutil.Git(t, repo, "rev-parse", "HEAD")
	s := quality.StepResult{Kinds: []quality.Kind{quality.Tests}, Provider: "vitest", Command: "vitest run", Status: quality.StatusPassed,
		Commit: head, Tests: &quality.TestCounts{Total: 3, Passed: 3}}
	if err := quality.SaveSession(repo, []quality.StepResult{s}); err != nil {
		t.Fatal(err)
	}
	env, _ := fakeAgentEnv(agentReply{out: claimingDraft})

	r, _ := prepare(t, repo, env, PRRequest{Agent: "claude", NoCheck: true})
	got := pruebas(t, r.Body)
	if got != "`vitest run`: 3 superadas, 0 fallidas." {
		t.Errorf("Pruebas = %q", got)
	}
}

func TestPreparePRAgentPruebasOtherLanguageAppends(t *testing.T) {
	repo := prRepo(t)
	m, err := manifest.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	m.Content.Language = "en"
	if err := manifest.Save(repo, m); err != nil {
		t.Fatal(err)
	}
	testutil.Git(t, repo, "commit", "-am", "chore: english content")
	env, _ := fakeAgentEnv(agentReply{out: claimingDraft})

	r, _ := prepare(t, repo, env, PRRequest{Agent: "claude", NoCheck: true})
	want := "## Pruebas\n\n- Todas las pruebas pasan correctamente en CI.\n\n- No se ejecutaron verificaciones automáticas para este commit.\n"
	if !strings.HasSuffix(r.Body, want) {
		t.Errorf("body does not end with %q:\n%s", want, r.Body)
	}
}

func TestReplaceSection(t *testing.T) {
	body := "## Resumen\n\nuno\n\n## pruebas\n\nviejo\nmás\n\n## Evidencias\n\nN/A\n"
	got := replaceSection(body, "Pruebas", func(string) string { return "- nuevo" })
	want := "## Resumen\n\nuno\n\n## pruebas\n\n- nuevo\n\n## Evidencias\n\nN/A\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	last := replaceSection("## Pruebas\nviejo\n", "Pruebas", func(c string) string { return c + "\n\n- nuevo" })
	if last != "## Pruebas\n\nviejo\n\n- nuevo\n" {
		t.Errorf("last section = %q", last)
	}
	missing := "## Resumen\n\nuno\n"
	if got := replaceSection(missing, "Pruebas", func(string) string { return "x" }); got != missing {
		t.Errorf("missing section changed the body: %q", got)
	}
}
