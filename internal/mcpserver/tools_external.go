package mcpserver

import (
	"context"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/google/jsonschema-go/jsonschema"
)

type prCreateInput struct {
	Draft        bool                 `json:"draft,omitempty" jsonschema:"Abre el PR como borrador."`
	EvidenceRefs []engine.EvidenceRef `json:"evidenceRefs,omitempty" jsonschema:"Referencias de Qapture para la sección Evidencias. Si cambian respecto del borrador guardado, se prepara uno nuevo; una lista vacía quita las evidencias."`
	Write        bool                 `json:"write,omitempty" jsonschema:"Debe ser true para hacer push y abrir o actualizar el PR en GitHub. Sin write solo devuelve el plan."`
}

// prCreateResult is the plan without write (or when the draft has problems)
// and the published report with write; status says which one it is.
type prCreateResult struct {
	Status   string `json:"status"`
	Question string `json:"question,omitempty"`
	*engine.CreatePlan
	*engine.CreateReport
}

func (prCreateResult) adjustSchema(s *jsonschema.Schema) { s.Required = []string{"status"} }

const (
	statusPublished      = "published"
	statusDraftProblems  = "draft-has-problems"
	draftProblemsNotSent = "pull request draft has problems; nothing was published"
)

func registerExternalTools(s *Server) {
	addTool(s, "pr_create", "Publica el PR en GitHub: hace push de la rama y abre o actualiza el pull request. Exige write: true; sin write devuelve el plan (push, PR existente y la pregunta de confirmación) sin publicar. Reutiliza el borrador de pr_prepare solo si HEAD, rama, target y evidencias coinciden; nunca publica un borrador con problemas.", EffectExternal,
		s.prCreate)
}

func (s *Server) prCreate(ctx context.Context, in prCreateInput, p Progress) (prCreateResult, int, error) {
	req := engine.CreateRequest{Draft: in.Draft, Prepare: engine.PRRequest{EvidenceRefs: in.EvidenceRefs}}
	plan, err := engine.PlanCreatePR(ctx, s.root, s.deps.Env, req, func(e engine.PREvent) { p(string(e.Stage), 0, 0) })
	if err != nil {
		return prCreateResult{}, 2, err
	}
	question := plan.Question()
	if len(plan.Report.Problems) > 0 {
		return prCreateResult{Status: statusDraftProblems, Question: question, CreatePlan: &plan}, 1, &resultError{draftProblemsNotSent}
	}
	if !in.Write {
		return prCreateResult{Status: statusNeedsWrite, Question: question, CreatePlan: &plan}, 0, nil
	}
	rep, err := engine.ApplyCreatePR(ctx, s.root, s.deps.Env, plan, req)
	if err != nil {
		return prCreateResult{}, 2, err
	}
	return prCreateResult{Status: statusPublished, CreateReport: &rep}, 0, nil
}
