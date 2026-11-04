package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/agents"
	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

// Paths of the report packs and their output, relative to the repo root.
const (
	ActivityGuide         = ".tija/skills/generar-reporte-actividades/SKILL.md"
	ActivitySchema        = ".tija/skills/generar-reporte-actividades/references/report-schema.md"
	ActivityCollectScript = ".tija/skills/generar-reporte-actividades/scripts/collect_activity.py"
	ActivityReportScript  = ".tija/skills/generar-reporte-actividades/scripts/build_activity_report.py"
	reportRequirements    = ".tija/skills/generar-informe-pdf/requirements.txt"
	reportsDir            = "docs/reportes"
	// SharedReportManifest, when present, sets the groups, repositories and
	// authors of every report; each report only replaces its period.
	SharedReportManifest = reportsDir + "/manifest.json"
)

const (
	dateLayout           = "2006-01-02"
	defaultScriptTimeout = 5 * time.Minute
	defaultReportTimeout = 5 * time.Minute
	reportAttempts       = 2
)

// ErrReportSetup marks a problem to fix before generating a report: an invalid
// period, missing packs or missing tools.
var ErrReportSetup = errors.New("report setup")

// setupError carries its own message and matches ErrReportSetup.
type setupError struct{ error }

func (setupError) Is(target error) bool { return target == ErrReportSetup }

// Unwrap keeps the cause (e.g. agents.ErrNotInstalled) visible to errors.Is.
func (e setupError) Unwrap() error { return e.error }

func setupErrorf(format string, args ...any) error {
	return setupError{fmt.Errorf(format, args...)}
}

// ReportRequest asks for the activity report of a period.
type ReportRequest struct {
	Since, Until  string // YYYY-MM-DD, inclusive
	Agent         string // "" uses the manifest default, "none" stops after collecting
	AgentTimeout  time.Duration
	ScriptTimeout time.Duration
	Now           time.Time // zero means time.Now
}

// ReportEvent announces the stage starting: collect, draft or build.
type ReportEvent struct {
	Stage string `json:"stage"`
}

// ReportResult reports what ActivityReport produced. Paths are repo-relative
// with '/'; PDF is empty when no report was generated.
type ReportResult struct {
	Since    string   `json:"since"`
	Until    string   `json:"until"`
	Folder   string   `json:"folder"`
	Activity string   `json:"activity,omitempty"`
	Report   string   `json:"report,omitempty"`
	PDF      string   `json:"pdf,omitempty"`
	Agent    string   `json:"agent,omitempty"`
	Commits  int      `json:"commits"`
	PRs      int      `json:"prs"`
	Warnings []string `json:"warnings"`
}

// HasReportPacks reports whether the activity report pack is installed in root.
func HasReportPacks(root string) bool {
	for _, rel := range []string{ActivityCollectScript, ActivityReportScript} {
		if _, err := os.Stat(abs(root, rel)); err != nil {
			return false
		}
	}
	return true
}

// ValidatePeriod checks that since and until are real YYYY-MM-DD dates in
// order and that until is not after today.
func ValidatePeriod(since, until string, today time.Time) error {
	a, err := time.Parse(dateLayout, since)
	if err != nil {
		return fmt.Errorf("since must be a date like 2026-10-01: %q", since)
	}
	b, err := time.Parse(dateLayout, until)
	if err != nil {
		return fmt.Errorf("until must be a date like 2026-10-31: %q", until)
	}
	if b.Before(a) {
		return fmt.Errorf("until (%s) is before since (%s)", until, since)
	}
	if until > today.Format(dateLayout) {
		return fmt.Errorf("until (%s) is in the future", until)
	}
	return nil
}

var monthsES = []string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto",
	"septiembre", "octubre", "noviembre", "diciembre"}

// periodLabel writes a validated period in Spanish, as the report shows it.
func periodLabel(since, until string) string {
	a, _ := time.Parse(dateLayout, since)
	b, _ := time.Parse(dateLayout, until)
	month := func(t time.Time) string { return monthsES[t.Month()-1] }
	switch {
	case since == until:
		return fmt.Sprintf("%d de %s de %d", a.Day(), month(a), a.Year())
	case a.Year() != b.Year():
		return fmt.Sprintf("%d de %s de %d al %d de %s de %d", a.Day(), month(a), a.Year(), b.Day(), month(b), b.Year())
	case a.Month() != b.Month():
		return fmt.Sprintf("%d de %s al %d de %s de %d", a.Day(), month(a), b.Day(), month(b), b.Year())
	}
	return fmt.Sprintf("%d al %d de %s de %d", a.Day(), b.Day(), month(b), b.Year())
}

// pythonCandidates lists the interpreters to try. On Windows "python" may be
// the alias that opens the Microsoft Store, so the launcher goes first.
func pythonCandidates() []string {
	if runtime.GOOS == "windows" {
		return []string{"py", "python", "python3"}
	}
	return []string{"python3", "python"}
}

// findPython returns an interpreter able to import ReportLab. Tija never
// installs it: the error says what to run.
func findPython(ctx context.Context, env sys.Env, root string) (string, error) {
	found := false
	for _, name := range pythonCandidates() {
		if _, err := env.LookPath(name); err != nil {
			continue
		}
		if _, _, code, err := env.Run(ctx, root, name, "-c", "pass"); err != nil || code != 0 {
			continue
		}
		found = true
		if _, _, code, err := env.Run(ctx, root, name, "-c", "import reportlab"); err == nil && code == 0 {
			return name, nil
		}
	}
	if found {
		return "", setupErrorf("ReportLab is not installed; run: pip install -r %s", reportRequirements)
	}
	return "", setupErrorf("Python 3 was not found on PATH")
}

// collectedActivity is the part of actividad.json that Tija reads.
type collectedActivity struct {
	Grupos []struct {
		Nombre    string  `json:"nombre"`
		TopeHoras float64 `json:"tope_horas"`
		Repos     []struct {
			Nombre       string            `json:"nombre"`
			Error        string            `json:"error"`
			AvisoPRs     string            `json:"aviso_prs"`
			Commits      []json.RawMessage `json:"commits"`
			PRs          []json.RawMessage `json:"prs"`
			SinCommitear []json.RawMessage `json:"sin_commitear"`
		} `json:"repos"`
	} `json:"grupos"`
}

// ActivityReport collects the work of the period, asks the agent to write the
// report as text and generates the PDF. The agent never touches the disk: Tija
// writes its answer, after replacing the facts it computed itself.
func ActivityReport(ctx context.Context, dir string, env sys.Env, req ReportRequest, emit func(ReportEvent)) (ReportResult, error) {
	if emit == nil {
		emit = func(ReportEvent) {}
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	res := ReportResult{Since: req.Since, Until: req.Until, Warnings: []string{}}
	if err := ValidatePeriod(req.Since, req.Until, now); err != nil {
		return res, setupError{err}
	}
	root, ok, err := gitx.FindRoot(ctx, env.Run, dir)
	if err != nil {
		return res, err
	}
	if !ok {
		return res, fmt.Errorf("%w: %s", ErrNotARepo, dir)
	}
	if !HasReportPacks(root) {
		return res, setupErrorf("the report packs are not installed; run tija init")
	}
	m, err := manifest.Load(root)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return res, setupError{err}
	}
	provider, err := reportProvider(m, env, req.Agent)
	if err != nil {
		return res, err
	}
	python, err := findPython(ctx, env, root)
	if err != nil {
		return res, err
	}

	if req.ScriptTimeout <= 0 {
		req.ScriptTimeout = defaultScriptTimeout
	}
	res.Folder = path.Join(reportsDir, "actividades-"+req.Since+"_"+req.Until)
	manifestPath := path.Join(res.Folder, "manifest.json")
	if err := writeReportManifest(root, manifestPath, m, req); err != nil {
		return res, err
	}

	emit(ReportEvent{Stage: "collect"})
	if _, err := runScript(ctx, env, root, python, req.ScriptTimeout, ActivityCollectScript, manifestPath, "--fetch"); err != nil {
		return res, fmt.Errorf("collect activity: %w", err)
	}
	data, err := os.ReadFile(abs(root, path.Join(res.Folder, "actividad.json")))
	if err != nil {
		return res, err
	}
	var activity collectedActivity
	if err := json.Unmarshal(data, &activity); err != nil {
		return res, fmt.Errorf("actividad.json: %w", err)
	}
	res.Activity = path.Join(res.Folder, "actividad.md")
	pending := 0
	for _, g := range activity.Grupos {
		for _, r := range g.Repos {
			res.Commits += len(r.Commits)
			res.PRs += len(r.PRs)
			pending += len(r.SinCommitear)
			if r.Error != "" {
				res.Warnings = append(res.Warnings, r.Nombre+": "+r.Error)
			}
			if r.AvisoPRs != "" {
				res.Warnings = append(res.Warnings, r.Nombre+": "+r.AvisoPRs)
			}
		}
	}
	if res.Commits+res.PRs+pending == 0 {
		res.Warnings = append(res.Warnings, "No activity found for your identities in this period.")
		return res, nil
	}
	if provider == "" {
		res.Warnings = append(res.Warnings, "No agent used: write reporte.json from "+res.Activity+
			" following "+ActivityGuide+", then run "+ActivityReportScript+".")
		return res, nil
	}
	res.Agent = provider

	prompt, err := reportPrompt(root, res, now)
	if err != nil {
		return res, err
	}
	tier := m.Agents.Tasks["report"].Tier
	if tier == "" {
		tier = "balanced"
	}
	timeout := req.AgentTimeout
	if timeout <= 0 {
		timeout = defaultReportTimeout
	}
	res.Report = path.Join(res.Folder, "reporte.json")
	pdf := path.Join(res.Folder, "reporte-actividades_"+req.Since+"_"+req.Until+".pdf")

	// An answer the report cannot use is a mistake the agent can correct, so
	// it gets one more attempt knowing what was wrong. A failing agent CLI
	// would only fail again.
	var problem error
	for range reportAttempts {
		attempt := prompt
		if problem != nil {
			attempt += "\n\nEl intento anterior no sirvió: " + problem.Error() + "\nCorrígelo y responde solo con el JSON."
		}
		emit(ReportEvent{Stage: "draft"})
		out, err := agents.Generate(ctx, env, provider, agents.Request{Prompt: attempt, Tier: tier, Dir: root, Timeout: timeout})
		if err != nil {
			return res, err
		}
		content, err := parseReport(out)
		if err != nil {
			problem = err
			continue
		}
		applyReportFacts(ctx, env, root, content, activity, req, now)
		if err := writeJSON(abs(root, res.Report), content); err != nil {
			return res, err
		}
		emit(ReportEvent{Stage: "build"})
		stderr, err := runScript(ctx, env, root, python, req.ScriptTimeout, ActivityReportScript, res.Report, "-o", pdf)
		if err != nil {
			problem = err
			continue
		}
		for _, line := range strings.Split(strings.ReplaceAll(stderr, "\r\n", "\n"), "\n") {
			if strings.HasPrefix(line, "AVISO") {
				res.Warnings = append(res.Warnings, line)
			}
		}
		res.PDF = pdf
		return res, nil
	}
	return res, fmt.Errorf("the agent did not produce a valid report: %w", problem)
}

// reportProvider resolves the agent; "" means none, by request or because no
// agent CLI is installed. An agent asked for by name must exist.
func reportProvider(m manifest.Manifest, env sys.Env, name string) (string, error) {
	switch name {
	case "none":
		return "", nil
	case "":
		if m.Agents.Default != "" && agents.Binary(m.Agents.Default, env.LookPath) != "" {
			return m.Agents.Default, nil
		}
		if avail := agents.Available(env.LookPath); len(avail) > 0 {
			return avail[0], nil
		}
		return "", nil
	}
	if agents.Binary(name, env.LookPath) == "" {
		return "", setupErrorf("%s: %w", name, agents.ErrNotInstalled)
	}
	return name, nil
}

// writeReportManifest writes the collector's input for this report: the shared
// manifest when there is one, else the current repository alone. Without
// "autores" the collector uses the identity of git and gh.
func writeReportManifest(root, rel string, m manifest.Manifest, req ReportRequest) error {
	content := map[string]any{}
	data, err := os.ReadFile(abs(root, SharedReportManifest))
	switch {
	case err == nil:
		// Editors on Windows may save with a UTF-8 BOM.
		if err := json.Unmarshal(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), &content); err != nil {
			return setupErrorf("%s: %w", SharedReportManifest, err)
		}
	case errors.Is(err, fs.ErrNotExist):
		name := m.Project.Name
		if name == "" {
			name = filepath.Base(root)
		}
		content["grupos"] = []any{map[string]any{"nombre": name, "repos": []any{map[string]any{"nombre": name, "path": "."}}}}
	default:
		return err
	}
	content["periodo"] = map[string]any{"desde": req.Since, "hasta": req.Until}

	if err := os.MkdirAll(abs(root, reportsDir), 0o755); err != nil {
		return err
	}
	// Reports name clients and hours: they stay out of git unless someone
	// deletes this file on purpose.
	ignore := abs(root, reportsDir+"/.gitignore")
	if _, err := os.Stat(ignore); errors.Is(err, fs.ErrNotExist) {
		if err := os.WriteFile(ignore, []byte("*\n"), 0o644); err != nil {
			return err
		}
	}
	return writeJSON(abs(root, rel), content)
}

// runScript runs a pack script with UTF-8 output and returns its stderr.
func runScript(ctx context.Context, env sys.Env, root, python string, timeout time.Duration, script string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	argv := append([]string{"-X", "utf8", script}, args...)
	_, stderr, code, err := env.Run(ctx, root, python, argv...)
	if ctx.Err() == context.DeadlineExceeded {
		return stderr, fmt.Errorf("%s timed out after %s", path.Base(script), timeout)
	}
	if err != nil {
		return stderr, err
	}
	if code != 0 {
		return stderr, fmt.Errorf("%s: %s", path.Base(script), lastLine(stderr))
	}
	return stderr, nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n")), "\n")
	return lines[len(lines)-1]
}

func reportPrompt(root string, res ReportResult, now time.Time) (string, error) {
	var parts [3]string
	for i, rel := range []string{ActivityGuide, ActivitySchema, res.Activity} {
		data, err := os.ReadFile(abs(root, rel))
		if err != nil {
			return "", err
		}
		parts[i] = string(data)
	}
	return fmt.Sprintf(`Redacta el contenido de reporte.json para un reporte de actividades del %s al %s. Hoy es %s.

No ejecutes comandos ni escribas archivos: la recolección ya está hecha y Tija guardará tu respuesta y generará el PDF.
Responde SOLO con el objeto JSON, sin texto alrededor. Incluye "resumen" y "grupos". Omite "meta": la completa Tija.
Usa como nombre de cada grupo el que aparece en la actividad recolectada. Incluye únicamente lo que respalde la actividad recolectada; no inventes trabajo, fechas ni horas.

<guia>
%s
</guia>

<formato>
%s
</formato>

<actividad_recolectada>
%s
</actividad_recolectada>
`, res.Since, res.Until, now.Format(dateLayout), parts[0], parts[1], parts[2]), nil
}

// parseReport takes the JSON object out of an agent answer, ignoring code
// fences and text around it, and checks the shape the report needs.
func parseReport(out string) (map[string]any, error) {
	start, end := strings.Index(out, "{"), strings.LastIndex(out, "}")
	if start < 0 || end < start {
		return nil, errors.New("the answer has no JSON object")
	}
	var content map[string]any
	if err := json.Unmarshal([]byte(out[start:end+1]), &content); err != nil {
		return nil, fmt.Errorf("the answer is not valid JSON: %w", err)
	}
	groups, _ := content["grupos"].([]any)
	if len(groups) == 0 {
		return nil, errors.New(`"grupos" is missing or empty`)
	}
	for _, g := range groups {
		group, _ := g.(map[string]any)
		if name, _ := group["nombre"].(string); name == "" {
			return nil, errors.New(`every group needs "nombre"`)
		}
		if projects, _ := group["proyectos"].([]any); len(projects) == 0 {
			return nil, errors.New(`every group needs "proyectos"`)
		}
	}
	return content, nil
}

// applyReportFacts overwrites what Tija knows and the agent must not decide:
// the document data and the hour limit measured by the collector.
func applyReportFacts(ctx context.Context, env sys.Env, root string, content map[string]any, activity collectedActivity, req ReportRequest, now time.Time) {
	meta := map[string]any{
		"title":        "Reporte de actividades",
		"periodo":      periodLabel(req.Since, req.Until),
		"nombre_corto": "ReporteActividades",
		"fecha":        now.Format(dateLayout),
		"version":      "1.0",
	}
	if stdout, _, code, err := env.Run(ctx, root, "git", "config", "user.name"); err == nil && code == 0 {
		if name := strings.TrimSpace(stdout); name != "" {
			meta["responsable"] = name
		}
	}
	content["meta"] = meta

	limits := map[string]float64{}
	for _, g := range activity.Grupos {
		limits[g.Nombre] = g.TopeHoras
	}
	groups, _ := content["grupos"].([]any)
	for _, g := range groups {
		group := g.(map[string]any)
		delete(group, "tope_horas")
		name, _ := group["nombre"].(string)
		if limit := limits[name]; limit > 0 {
			group["tope_horas"] = limit
		}
	}
}

func writeJSON(p string, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, buf.Bytes(), 0o644)
}
