package mcpserver

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
)

type qualityCheckInput struct {
	Kinds      []string `json:"kinds,omitempty" jsonschema:"Tipos de comprobación: lint, format o test; omitido ejecuta todos como el CLI check."`
	TimeoutSec *int     `json:"timeoutSec,omitempty" jsonschema:"Timeout positivo por proceso, en segundos; por defecto 600."`
}

type reviewRunInput struct {
	Mode         string `json:"mode,omitempty" jsonschema:"review, quality o deep; por defecto review."`
	FailOn       string `json:"failOn,omitempty" jsonschema:"Umbral: critical, high, medium, low o none."`
	Agent        string `json:"agent,omitempty" jsonschema:"claude, codex, gemini o none; none ejecuta solo comprobaciones deterministas; omitido usa el default del CLI."`
	NoCheck      bool   `json:"noCheck,omitempty"`
	NoPRComments bool   `json:"noPRComments,omitempty"`
}

type prPrepareInput struct {
	Agent        string               `json:"agent,omitempty" jsonschema:"claude, codex, gemini o none; none genera un borrador determinista; omitido usa el default del CLI."`
	NoCheck      bool                 `json:"noCheck,omitempty"`
	EvidenceRefs []engine.EvidenceRef `json:"evidenceRefs,omitempty" jsonschema:"Referencias estructuradas de Qapture. Tija no comprueba su existencia; de exportPath solo conserva el nombre del archivo."`
}

func validateLocalAgent(name string) error {
	if !slices.Contains([]string{"", "claude", "codex", "gemini", "none"}, name) {
		return fmt.Errorf("unknown agent (use claude, codex, gemini or none)")
	}
	return nil
}

func registerLocalTools(s *Server) {
	addTool(s, "quality_check", "Ejecuta las comprobaciones de calidad; solo escribe resultados persistentes en .tija/session/ y artefactos efímeros de calidad; nunca toca archivos versionados.", EffectLocal,
		func(ctx context.Context, in qualityCheckInput, p Progress) (engine.QualityReport, int, error) {
			var kinds []quality.Kind
			if in.Kinds != nil {
				if len(in.Kinds) == 0 {
					return engine.QualityReport{}, 2, fmt.Errorf("kinds must not be empty")
				}
				for _, kind := range in.Kinds {
					switch kind {
					case "lint":
						kinds = append(kinds, quality.Lint)
					case "format":
						kinds = append(kinds, quality.Format)
					case "test":
						kinds = append(kinds, quality.Tests)
					default:
						return engine.QualityReport{}, 2, fmt.Errorf("invalid quality kind %q: expected lint, format or test", kind)
					}
				}
			}
			timeout := engine.DefaultCheckTimeout
			if in.TimeoutSec != nil {
				if *in.TimeoutSec <= 0 || int64(*in.TimeoutSec) > int64((time.Duration(1<<63-1))/time.Second) {
					return engine.QualityReport{}, 2, fmt.Errorf("timeoutSec must be a positive duration in seconds")
				}
				timeout = time.Duration(*in.TimeoutSec) * time.Second
			}
			r, err := engine.RunQuality(ctx, s.root, s.deps.Env, engine.QualityRequest{Kinds: kinds, Mode: quality.CheckMode, Timeout: timeout}, func(e engine.QualityEvent) { p("quality", e.Index+1, e.Total) })
			return r, r.ExitCode(), err
		})
	addTool(s, "review_run", "Revisa los cambios con el mismo motor del CLI; solo escribe en .tija/session/ y usa temporales externos; nunca toca archivos versionados. agent: none deja solo comprobaciones deterministas.", EffectLocal,
		func(ctx context.Context, in reviewRunInput, p Progress) (engine.ReviewReport, int, error) {
			if err := validateLocalAgent(in.Agent); err != nil {
				return engine.ReviewReport{}, 2, err
			}
			if in.Mode != "" && !slices.Contains([]string{"review", "quality", "deep"}, in.Mode) {
				return engine.ReviewReport{}, 2, fmt.Errorf("invalid review mode %q", in.Mode)
			}
			if in.FailOn != "" && !slices.Contains(manifest.FailOnValues, in.FailOn) {
				return engine.ReviewReport{}, 2, fmt.Errorf("invalid review.failOn %q", in.FailOn)
			}
			r, err := engine.RunReview(ctx, s.root, s.deps.Env, engine.ReviewRequest{Mode: in.Mode, FailOn: in.FailOn, Agent: in.Agent, NoCheck: in.NoCheck, NoPRComments: in.NoPRComments}, func(e engine.ReviewEvent) { p(string(e.Stage), e.Chunk, e.Chunks) })
			return r, r.ExitCode(), err
		})
	addTool(s, "pr_prepare", "Prepara un borrador de PR; solo escribe en .tija/session/ y artefactos efímeros; nunca toca archivos versionados ni publica. agent: none produce un borrador determinista. Revisa los datos de evidencias antes de publicarlos.", EffectLocal,
		func(ctx context.Context, in prPrepareInput, p Progress) (engine.PRReport, int, error) {
			if err := validateLocalAgent(in.Agent); err != nil {
				return engine.PRReport{}, 2, err
			}
			r, err := engine.PreparePR(ctx, s.root, s.deps.Env, engine.PRRequest{Agent: in.Agent, NoCheck: in.NoCheck, EvidenceRefs: in.EvidenceRefs}, func(e engine.PREvent) { p(string(e.Stage), 0, 0) })
			return r, r.ExitCode(), err
		})
}
