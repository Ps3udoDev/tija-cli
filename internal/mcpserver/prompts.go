package mcpserver

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed prompts/documentar_pr.md
var documentarPRPrompt string

func registerPrompts(s *Server) {
	s.MCP.AddPrompt(&mcp.Prompt{
		Name:        "documentar_pr",
		Description: "Guía la revisión y preparación de un PR con evidencias de Qapture; publicar requiere una solicitud explícita del usuario.",
		Arguments: []*mcp.PromptArgument{
			{Name: "qaptureProject", Description: "Nombre o id del proyecto de Qapture; omitido usa el que indique el tester."},
			{Name: "mode", Description: "Modo de revisión: review, quality o deep; por defecto review."},
		},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		mode := req.Params.Arguments["mode"]
		if mode == "" {
			mode = "review"
		}
		if mode != "review" && mode != "quality" && mode != "deep" {
			return nil, fmt.Errorf("invalid review mode %q: expected review, quality or deep", mode)
		}
		project := req.Params.Arguments["qaptureProject"]
		if project == "" {
			project = "el que indique el tester"
		} else {
			// Quoting keeps a project argument on one line and identifies it as
			// data. NewReplacer substitutes once, never interpreting argument text
			// as another template variable or evaluating it as instructions.
			project = strconv.Quote(project)
		}
		text := strings.NewReplacer("{{mode}}", mode, "{{qaptureProject}}", project).Replace(documentarPRPrompt)
		return &mcp.GetPromptResult{
			Description: "Preparar un PR con evidencias de QA verificadas en Qapture.",
			Messages:    []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: text}}},
		}, nil
	})
}
