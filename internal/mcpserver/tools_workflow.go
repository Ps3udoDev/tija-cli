package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/workflows"
	"github.com/google/jsonschema-go/jsonschema"
)

type workflowRunInput struct {
	Name    string `json:"name" jsonschema:"Nombre del workflow de .tija/workflows/ (por ejemplo pre-pr)."`
	Write   bool   `json:"write,omitempty" jsonschema:"Autoriza los pasos que modifican archivos del repositorio y los que publican."`
	Unsafe  bool   `json:"unsafe,omitempty" jsonschema:"Autoriza los pasos con --unsafe (requiere write)."`
	Publish bool   `json:"publish,omitempty" jsonschema:"Autoriza los pasos pr create, que hacen push y publican en GitHub (requiere write)."`
}

// workflowRunResult says whether the workflow ran. needs-permission lists
// what is missing and every step with its effect, and nothing ran; completed
// carries the same report as tija run --json.
type workflowRunResult struct {
	Status  string                 `json:"status"`
	Needs   []string               `json:"needs,omitempty"`
	Planned []workflows.StepEffect `json:"plannedSteps,omitempty"`
	*workflows.WorkflowReport
}

func (workflowRunResult) adjustSchema(s *jsonschema.Schema) { s.Required = []string{"status"} }

const (
	statusNeedsPermission = "needs-permission"
	statusCompleted       = "completed"
)

func registerWorkflowTool(s *Server) {
	addTool(s, "workflow_run", "Ejecuta un workflow de .tija/workflows/ con los comandos permitidos de Tija. Antes de correr clasifica cada paso: los de solo lectura no piden nada; los que modifican el repositorio exigen write: true (y unsafe si usan --unsafe); los que publican un PR exigen además publish: true. Si falta un permiso no ejecuta ningún paso y devuelve status needs-permission con la lista.", EffectExternal,
		s.workflowRun)
}

func (s *Server) workflowRun(ctx context.Context, in workflowRunInput, p Progress) (workflowRunResult, int, error) {
	if s.deps.RunStep == nil {
		return workflowRunResult{}, 2, errors.New("workflow_run is not available in this server")
	}
	w, err := s.loadWorkflow(in.Name)
	if err != nil {
		return workflowRunResult{}, 2, err
	}
	needs, err := workflows.Requirements(w)
	if err != nil {
		return workflowRunResult{}, 2, err
	}
	var missing []string
	if needs.Write && !in.Write {
		missing = append(missing, "write")
	}
	if needs.Unsafe && !in.Unsafe {
		missing = append(missing, "unsafe")
	}
	if needs.Publish && !in.Publish {
		missing = append(missing, "publish")
	}
	if len(missing) > 0 {
		return workflowRunResult{Status: statusNeedsPermission, Needs: missing, Planned: needs.Steps}, 0, nil
	}
	rep := workflows.Run(ctx, w, s.deps.RunStep, workflows.Options{JSON: true, Yes: true,
		OnStep: func(i int, r workflows.StepResult) { p(r.Run, i+1, len(w.Steps)) }})
	out := workflowRunResult{Status: statusCompleted, WorkflowReport: &rep}
	if rep.Exit >= 2 {
		return out, rep.Exit, &resultError{fmt.Sprintf("workflow %s stopped: a step could not complete (exit %d)", w.Name, rep.Exit)}
	}
	return out, rep.Exit, nil
}

// loadWorkflow finds and validates the named workflow under the server root.
func (s *Server) loadWorkflow(name string) (workflows.Workflow, error) {
	if strings.TrimSpace(name) == "" {
		return workflows.Workflow{}, errors.New("name is required")
	}
	wfs, loadErrs := workflows.Load(s.root)
	for _, w := range wfs {
		if w.Name != name {
			continue
		}
		if errs := workflows.Validate(w); len(errs) > 0 {
			return workflows.Workflow{}, fmt.Errorf("workflow %q is invalid: %w", name, errs[0])
		}
		return w, nil
	}
	for _, err := range loadErrs {
		if err != nil {
			return workflows.Workflow{}, fmt.Errorf("workflow %q not found; a workflow file is invalid: %w", name, err)
		}
	}
	return workflows.Workflow{}, fmt.Errorf("unknown workflow %q", name)
}
