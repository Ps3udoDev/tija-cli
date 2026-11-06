package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
	"github.com/google/jsonschema-go/jsonschema"
)

// statusNeedsWrite marks a result that describes a change without making it.
const statusNeedsWrite = "needs-write"

type qualityFixInput struct {
	Kinds []string `json:"kinds,omitempty" jsonschema:"Tipos a corregir: lint o format; omitido corrige ambos, como tija lint --fix."`
	Write bool     `json:"write,omitempty" jsonschema:"Debe ser true para modificar archivos del repositorio. Sin write no se ejecuta nada."`
}

// qualityFixResult is the engine report when the fix ran; without write it
// only says what would run, so the report fields are absent.
type qualityFixResult struct {
	Status string   `json:"status"`
	Would  []string `json:"would,omitempty"`
	*engine.QualityReport
}

type commentsCleanInput struct {
	Paths  []string `json:"paths,omitempty" jsonschema:"Rutas relativas a la raíz; omitido limpia los archivos del diff."`
	All    bool     `json:"all,omitempty" jsonschema:"Limpia todos los archivos soportados del repositorio."`
	Agent  string   `json:"agent,omitempty" jsonschema:"claude, codex, gemini o none; none aplica solo las reglas deterministas; omitido usa el default del CLI."`
	Write  bool     `json:"write,omitempty" jsonschema:"Debe ser true para escribir; sin write devuelve el plan con su diff y no toca archivos."`
	Unsafe bool     `json:"unsafe,omitempty" jsonschema:"Aplica también cambios inseguros (código comentado, reescrituras). Requiere write: true."`
}

func registerRepoTools(s *Server) {
	addTool(s, "quality_fix", "Corrige lint y formato con las herramientas del repositorio. Modifica archivos versionados: exige write: true; sin write solo devuelve lo que ejecutaría.", EffectRepo,
		s.qualityFix)
	addTool(s, "comments_clean", "Limpia comentarios ruidosos. Sin write devuelve el plan y su diff sin tocar archivos; con write: true modifica archivos versionados, y unsafe: true agrega los cambios inseguros.", EffectRepo,
		func(ctx context.Context, in commentsCleanInput, p Progress) (engine.CleanReport, int, error) {
			if err := validateLocalAgent(in.Agent); err != nil {
				return engine.CleanReport{}, 2, err
			}
			if in.Unsafe && !in.Write {
				return engine.CleanReport{}, 2, fmt.Errorf("unsafe requires write: true")
			}
			req := engine.CleanRequest{Paths: in.Paths, All: in.All, Write: in.Write, Unsafe: in.Unsafe, Diff: !in.Write, Agent: in.Agent}
			if in.Agent == "none" {
				req.NoAgent = true
			}
			r, err := engine.CleanComments(ctx, s.root, s.deps.Env, req, func(e engine.CleanEvent) { p(e.Type, e.Done, e.Total) })
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return engine.CleanReport{}, 2, cancelledError(nil)
			}
			return r, r.ExitCode(), err
		})
}

// fixKinds maps the public kind names to the engine's; omitted means lint
// and format, the set tija lint --fix runs.
func fixKinds(in []string) ([]quality.Kind, []string, error) {
	if in == nil {
		return []quality.Kind{quality.Lint, quality.Format}, []string{"lint", "format"}, nil
	}
	if len(in) == 0 {
		return nil, nil, fmt.Errorf("kinds must not be empty")
	}
	var kinds []quality.Kind
	for _, k := range in {
		switch k {
		case "lint":
			kinds = append(kinds, quality.Lint)
		case "format":
			kinds = append(kinds, quality.Format)
		default:
			return nil, nil, fmt.Errorf("invalid fix kind %q: expected lint or format", k)
		}
	}
	return kinds, in, nil
}

func cancelledError(changed []string) error {
	if len(changed) == 0 {
		return errors.New("cancelled; no files were changed")
	}
	return fmt.Errorf("cancelled; changed files: %s", strings.Join(changed, ", "))
}

// qualityFix runs lint and format fixes; it is a method so the cancellation
// path can be exercised without a client, which never sees a cancelled reply.
func (s *Server) qualityFix(ctx context.Context, in qualityFixInput, p Progress) (qualityFixResult, int, error) {
	kinds, names, err := fixKinds(in.Kinds)
	if err != nil {
		return qualityFixResult{}, 2, err
	}
	if !in.Write {
		return qualityFixResult{Status: statusNeedsWrite, Would: names}, 0, nil
	}
	r, err := engine.RunQuality(ctx, s.root, s.deps.Env, engine.QualityRequest{Kinds: kinds, Mode: quality.FixMode},
		func(e engine.QualityEvent) { p("fix", e.Index+1, e.Total) })
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return qualityFixResult{}, 2, cancelledError(r.ChangedFiles)
	}
	if err != nil {
		return qualityFixResult{}, 2, err
	}
	return qualityFixResult{Status: "applied", QualityReport: &r}, r.ExitCode(), nil
}

// adjustSchema keeps only status required: without write the engine report
// is absent, and with write its fields are present exactly as in the CLI JSON.
func (qualityFixResult) adjustSchema(s *jsonschema.Schema) { s.Required = []string{"status"} }
