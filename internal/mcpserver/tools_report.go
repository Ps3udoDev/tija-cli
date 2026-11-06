package mcpserver

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type reportActivityInput struct {
	Since           string `json:"since" jsonschema:"Inicio del período, inclusive, YYYY-MM-DD."`
	Until           string `json:"until" jsonschema:"Fin del período, inclusive, YYYY-MM-DD; no anterior a since ni posterior a hoy."`
	Agent           string `json:"agent,omitempty" jsonschema:"claude, codex, gemini o none; none solo recolecta. Omitido usa el default del CLI."`
	AgentTimeoutSec *int   `json:"agentTimeoutSec,omitempty" jsonschema:"Timeout positivo del agente en segundos; por defecto 300."`
}

func activityReportRequest(in reportActivityInput) (engine.ReportRequest, error) {
	if err := engine.ValidatePeriod(in.Since, in.Until, time.Now()); err != nil {
		return engine.ReportRequest{}, fmt.Errorf("invalid dates: %w", err)
	}
	if err := validateLocalAgent(in.Agent); err != nil {
		return engine.ReportRequest{}, err
	}
	timeout := 300 * time.Second
	if in.AgentTimeoutSec != nil {
		if *in.AgentTimeoutSec <= 0 || int64(*in.AgentTimeoutSec) > int64((time.Duration(1<<63-1))/time.Second) {
			return engine.ReportRequest{}, fmt.Errorf("agentTimeoutSec must be a positive duration in seconds")
		}
		timeout = time.Duration(*in.AgentTimeoutSec) * time.Second
	}
	return engine.ReportRequest{Since: in.Since, Until: in.Until, Agent: in.Agent, AgentTimeout: timeout}, nil
}

func registerReportActivityTool(s *Server) {
	a := annotations(EffectLocal)
	openWorld := true // The collector reads pull requests using gh.
	a.OpenWorldHint = &openWorld
	schema, err := outputSchema(reflect.TypeFor[engine.ReportResult]())
	if err != nil {
		panic(fmt.Sprintf("output schema for report_activity: %v", err))
	}
	mcp.AddTool(s.MCP, &mcp.Tool{
		Name: "report_activity", Annotations: a, OutputSchema: schema,
		Description: "Genera un reporte de actividades; solo escribe en docs/reportes/, ignorado por Git. Lee PRs con gh y requiere los packs de reportes y Python con ReportLab. agent: none solo recolecta; nunca modifica archivos versionados ni publica.",
	}, toolHandler(s, "report_activity", EffectLocal, func(ctx context.Context, in reportActivityInput, p Progress) (engine.ReportResult, int, error) {
		req, err := activityReportRequest(in)
		if err != nil {
			return engine.ReportResult{}, 2, err
		}
		r, err := engine.ActivityReport(ctx, s.root, s.deps.Env, req, func(e engine.ReportEvent) { p(e.Stage, 0, 0) })
		if err != nil {
			// Collection or generation may already have created local artifacts.
			// Preserve the engine's paths instead of claiming no files changed.
			return r, 2, &resultError{msg: err.Error()}
		}
		return r, 0, nil
	}))
}
