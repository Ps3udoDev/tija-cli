package engine

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

var reportToday = time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)

func TestValidatePeriod(t *testing.T) {
	for _, tt := range []struct {
		since, until string
		want         string
	}{
		{"2026-10-01", "2026-10-31", ""},
		{"2026-11-15", "2026-11-15", ""},
		{"2026-10-1", "2026-10-31", "since"},
		{"2026-02-30", "2026-03-01", "since"},
		{"2026-10-01", "", "until"},
		{"2026-10-31", "2026-10-01", "before"},
		{"2026-11-01", "2026-11-16", "future"},
	} {
		err := ValidatePeriod(tt.since, tt.until, reportToday)
		switch {
		case tt.want == "" && err != nil:
			t.Errorf("ValidatePeriod(%q, %q) = %v", tt.since, tt.until, err)
		case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
			t.Errorf("ValidatePeriod(%q, %q) = %v, want %q", tt.since, tt.until, err, tt.want)
		}
	}
}

func TestPeriodLabel(t *testing.T) {
	for _, tt := range []struct{ since, until, want string }{
		{"2026-10-01", "2026-10-31", "1 al 31 de octubre de 2026"},
		{"2026-10-05", "2026-10-05", "5 de octubre de 2026"},
		{"2026-10-28", "2026-11-03", "28 de octubre al 3 de noviembre de 2026"},
		{"2025-12-20", "2026-01-10", "20 de diciembre de 2025 al 10 de enero de 2026"},
	} {
		if got := periodLabel(tt.since, tt.until); got != tt.want {
			t.Errorf("periodLabel(%q, %q) = %q, want %q", tt.since, tt.until, got, tt.want)
		}
	}
}

// reportFake simulates python and the agent; git still runs for real.
type reportFake struct {
	repo      string
	activity  string   // actividad.json written by the collector
	answers   []string // agent answers, in order
	buildFail int      // number of builds that fail
	noPython  bool
	noLib     bool
	noAgent   bool

	builds  int
	prompts []string
	scripts []string
}

func (f *reportFake) env() sys.Env {
	real := sys.Default()
	return sys.Env{
		Getenv: func(string) string { return "" },
		LookPath: func(name string) (string, error) {
			switch {
			case name == "git":
				return "git", nil
			case isPython(name):
				if f.noPython {
					return "", errors.New("not found")
				}
				return name, nil
			case name == "claude" && !f.noAgent:
				return "claude", nil
			}
			return "", errors.New("not found")
		},
		Run: func(ctx context.Context, dir, name string, args ...string) (string, string, int, error) {
			if !isPython(name) {
				return real.Run(ctx, dir, name, args...)
			}
			return f.python(dir, args)
		},
		RunInput: func(_ context.Context, _, stdin, _ string, _ ...string) (string, string, int, error) {
			f.prompts = append(f.prompts, stdin)
			if len(f.answers) == 0 {
				return "", "agent failed", 1, nil
			}
			out := f.answers[0]
			f.answers = f.answers[1:]
			return out, "", 0, nil
		},
	}
}

func isPython(name string) bool { return name == "python" || name == "python3" || name == "py" }

func (f *reportFake) python(dir string, args []string) (string, string, int, error) {
	if args[0] == "-c" {
		if strings.Contains(args[1], "reportlab") && f.noLib {
			return "", "No module named reportlab", 1, nil
		}
		return "", "", 0, nil
	}
	script := args[2]
	f.scripts = append(f.scripts, filepath.Base(script))
	switch filepath.Base(script) {
	case "collect_activity.py":
		folder := filepath.Dir(filepath.Join(dir, args[3]))
		if err := os.WriteFile(filepath.Join(folder, "actividad.json"), []byte(f.activity), 0o644); err != nil {
			return "", "", 1, err
		}
		if err := os.WriteFile(filepath.Join(folder, "actividad.md"), []byte("# Actividad de prueba\n- feat: algo"), 0o644); err != nil {
			return "", "", 1, err
		}
	case "build_activity_report.py":
		f.builds++
		if f.builds <= f.buildFail {
			return "", "Traceback\nValueError: estado 'Hecho' invalido", 1, nil
		}
		out := args[slices.Index(args, "-o")+1]
		if err := os.WriteFile(filepath.Join(dir, out), []byte("%PDF"), 0o644); err != nil {
			return "", "", 1, err
		}
		return "", "AVISO: ACME: 9 h superan el tope de 5 h del recolector", 0, nil
	}
	return "", "", 0, nil
}

const activityWithWork = `{"grupos":[{"nombre":"demo","tope_horas":5.5,"repos":[{"nombre":"demo","commits":[{"sha":"a"},{"sha":"b"}],"prs":[{"numero":1}],"sin_commitear":[],"aviso_prs":"gh pr list fallo"}]}]}`
const activityEmpty = `{"grupos":[{"nombre":"demo","tope_horas":0,"repos":[{"nombre":"demo","commits":[],"prs":[],"sin_commitear":[]}]}]}`
const goodReport = "Aquí está:\n```json\n" + `{"meta":{"title":"Inventado","periodo":"otro"},"grupos":[{"nombre":"demo","tope_horas":99,"proyectos":[{"nombre":"demo","actividades":[{"desde":"2026-10-02","actividad":"A","descripcion":"B","horas":1,"fuente":"estimado","estado":"Completado"}]}]}]}` + "\n```"

func reportRepo(t *testing.T) string {
	t.Helper()
	repo := testutil.NewRepo(t, fixture("unknown"))
	for _, rel := range []string{ActivityReportScript, ActivityCollectScript, ActivityGuide, ActivitySchema} {
		p := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("contenido de "+filepath.Base(rel)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func runReport(t *testing.T, f *reportFake, req ReportRequest) (ReportResult, error, []string) {
	t.Helper()
	req.Since, req.Until, req.Now = "2026-10-01", "2026-10-31", reportToday
	var events []string
	res, err := ActivityReport(context.Background(), f.repo, f.env(), req, func(ev ReportEvent) { events = append(events, ev.Stage) })
	return res, err, events
}

func TestActivityReportWritesPDF(t *testing.T) {
	f := &reportFake{repo: reportRepo(t), activity: activityWithWork, answers: []string{goodReport}}
	res, err, events := runReport(t, f, ReportRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(events, ","), "collect,draft,build"; got != want {
		t.Errorf("events = %q, want %q", got, want)
	}
	folder := "docs/reportes/actividades-2026-10-01_2026-10-31"
	if res.PDF != folder+"/reporte-actividades_2026-10-01_2026-10-31.pdf" || res.Agent != "claude" {
		t.Errorf("result = %+v", res)
	}
	if res.Commits != 2 || res.PRs != 1 {
		t.Errorf("commits = %d, prs = %d", res.Commits, res.PRs)
	}
	joined := strings.Join(res.Warnings, "\n")
	if !strings.Contains(joined, "gh pr list fallo") || !strings.Contains(joined, "superan el tope") {
		t.Errorf("warnings = %q", joined)
	}

	prompt := f.prompts[0]
	for _, want := range []string{"contenido de SKILL.md", "contenido de report-schema.md", "feat: algo", "2026-10-01"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt misses %q", want)
		}
	}

	// Tija owns the facts it computed: the agent cannot change them.
	var content map[string]any
	data, err := os.ReadFile(filepath.Join(f.repo, filepath.FromSlash(folder), "reporte.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &content); err != nil {
		t.Fatal(err)
	}
	meta := content["meta"].(map[string]any)
	if meta["periodo"] != "1 al 31 de octubre de 2026" || meta["title"] != "Reporte de actividades" || meta["fecha"] != "2026-11-15" {
		t.Errorf("meta = %v", meta)
	}
	group := content["grupos"].([]any)[0].(map[string]any)
	if group["tope_horas"] != 5.5 {
		t.Errorf("tope_horas = %v, want 5.5", group["tope_horas"])
	}
}

func TestActivityReportWithoutWorkSkipsAgent(t *testing.T) {
	f := &reportFake{repo: reportRepo(t), activity: activityEmpty, answers: []string{goodReport}}
	res, err, _ := runReport(t, f, ReportRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.prompts) != 0 || res.PDF != "" {
		t.Errorf("prompts = %d, pdf = %q", len(f.prompts), res.PDF)
	}
	if !strings.Contains(strings.Join(res.Warnings, "\n"), "No activity") {
		t.Errorf("warnings = %v", res.Warnings)
	}
}

func TestActivityReportRetriesInvalidAnswers(t *testing.T) {
	t.Run("not JSON", func(t *testing.T) {
		f := &reportFake{repo: reportRepo(t), activity: activityWithWork, answers: []string{"no puedo", goodReport}}
		res, err, _ := runReport(t, f, ReportRequest{})
		if err != nil || res.PDF == "" {
			t.Fatalf("res = %+v, err = %v", res, err)
		}
		if len(f.prompts) != 2 || !strings.Contains(f.prompts[1], "intento anterior") {
			t.Errorf("prompts = %v", f.prompts)
		}
	})
	t.Run("rejected by the build", func(t *testing.T) {
		f := &reportFake{repo: reportRepo(t), activity: activityWithWork, answers: []string{goodReport, goodReport}, buildFail: 1}
		res, err, _ := runReport(t, f, ReportRequest{})
		if err != nil || res.PDF == "" {
			t.Fatalf("res = %+v, err = %v", res, err)
		}
		if !strings.Contains(f.prompts[1], "estado 'Hecho' invalido") {
			t.Errorf("retry prompt misses the error: %q", f.prompts[1])
		}
	})
	t.Run("gives up after two", func(t *testing.T) {
		f := &reportFake{repo: reportRepo(t), activity: activityWithWork, answers: []string{"a", "b", goodReport}}
		res, err, _ := runReport(t, f, ReportRequest{})
		if err == nil || res.PDF != "" || len(f.prompts) != 2 {
			t.Errorf("res = %+v, err = %v, prompts = %d", res, err, len(f.prompts))
		}
		if res.Activity == "" {
			t.Error("the collected activity must stay available after a failure")
		}
	})
}

func TestActivityReportWithoutAgentStopsAfterCollecting(t *testing.T) {
	for name, tt := range map[string]struct {
		f   *reportFake
		req ReportRequest
	}{
		"no-agent":  {&reportFake{activity: activityWithWork}, ReportRequest{Agent: "none"}},
		"not found": {&reportFake{activity: activityWithWork, noAgent: true}, ReportRequest{}},
	} {
		t.Run(name, func(t *testing.T) {
			tt.f.repo = reportRepo(t)
			res, err, _ := runReport(t, tt.f, tt.req)
			if err != nil {
				t.Fatal(err)
			}
			if res.PDF != "" || !strings.HasSuffix(res.Activity, "actividad.md") {
				t.Errorf("result = %+v", res)
			}
			if !strings.Contains(strings.Join(res.Warnings, "\n"), "reporte.json") {
				t.Errorf("warnings = %v", res.Warnings)
			}
		})
	}
}

func TestActivityReportSetupErrors(t *testing.T) {
	t.Run("named agent missing", func(t *testing.T) {
		f := &reportFake{repo: reportRepo(t), activity: activityWithWork}
		if _, err, _ := runReport(t, f, ReportRequest{Agent: "codex"}); !errors.Is(err, ErrReportSetup) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("packs missing", func(t *testing.T) {
		f := &reportFake{repo: testutil.NewRepo(t, fixture("unknown"))}
		_, err, _ := runReport(t, f, ReportRequest{})
		if !errors.Is(err, ErrReportSetup) || !strings.Contains(err.Error(), "tija init") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("python missing", func(t *testing.T) {
		f := &reportFake{repo: reportRepo(t), noPython: true}
		_, err, _ := runReport(t, f, ReportRequest{})
		if !errors.Is(err, ErrReportSetup) || !strings.Contains(err.Error(), "Python") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("reportlab missing", func(t *testing.T) {
		f := &reportFake{repo: reportRepo(t), noLib: true}
		_, err, _ := runReport(t, f, ReportRequest{})
		if !errors.Is(err, ErrReportSetup) || !strings.Contains(err.Error(), "pip install -r") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("invalid period", func(t *testing.T) {
		f := &reportFake{repo: reportRepo(t)}
		_, err := ActivityReport(context.Background(), f.repo, f.env(), ReportRequest{Since: "x", Until: "y", Now: reportToday}, nil)
		if !errors.Is(err, ErrReportSetup) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestActivityReportManifest(t *testing.T) {
	read := func(t *testing.T, repo string) map[string]any {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(repo, "docs/reportes/actividades-2026-10-01_2026-10-31/manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	period := map[string]any{"desde": "2026-10-01", "hasta": "2026-10-31"}

	t.Run("current repository by default", func(t *testing.T) {
		f := &reportFake{repo: reportRepo(t), activity: activityEmpty}
		if _, err, _ := runReport(t, f, ReportRequest{}); err != nil {
			t.Fatal(err)
		}
		m := read(t, f.repo)
		if m["autores"] != nil || len(m["grupos"].([]any)) != 1 {
			t.Errorf("manifest = %v", m)
		}
		if p := m["periodo"].(map[string]any); p["desde"] != period["desde"] || p["hasta"] != period["hasta"] {
			t.Errorf("periodo = %v", p)
		}
	})

	t.Run("shared manifest keeps its groups and takes the period", func(t *testing.T) {
		f := &reportFake{repo: reportRepo(t), activity: activityEmpty}
		shared := filepath.Join(f.repo, filepath.FromSlash(SharedReportManifest))
		if err := os.MkdirAll(filepath.Dir(shared), 0o755); err != nil {
			t.Fatal(err)
		}
		content := `{"periodo":{"desde":"2000-01-01","hasta":"2000-01-02"},"autores":["ana"],"grupos":[{"nombre":"A","repos":[]},{"nombre":"B","repos":[]}]}`
		if err := os.WriteFile(shared, []byte("\xef\xbb\xbf"+content), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err, _ := runReport(t, f, ReportRequest{}); err != nil {
			t.Fatal(err)
		}
		m := read(t, f.repo)
		if p := m["periodo"].(map[string]any); p["desde"] != "2026-10-01" {
			t.Errorf("periodo = %v", p)
		}
		if len(m["grupos"].([]any)) != 2 || m["autores"].([]any)[0] != "ana" {
			t.Errorf("manifest = %v", m)
		}
	})

	t.Run("invalid shared manifest is a setup error", func(t *testing.T) {
		f := &reportFake{repo: reportRepo(t), activity: activityEmpty}
		shared := filepath.Join(f.repo, filepath.FromSlash(SharedReportManifest))
		if err := os.MkdirAll(filepath.Dir(shared), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(shared, []byte("{"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err, _ := runReport(t, f, ReportRequest{}); !errors.Is(err, ErrReportSetup) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestPythonCandidates(t *testing.T) {
	got := pythonCandidates()
	if runtime.GOOS == "windows" {
		if got[0] != "py" {
			t.Errorf("candidates = %v, want py first: python may open the Microsoft Store", got)
		}
	} else if got[0] != "python3" {
		t.Errorf("candidates = %v, want python3 first", got)
	}
}
