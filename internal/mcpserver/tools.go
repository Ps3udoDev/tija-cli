package mcpserver

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Effect describes the scope of a tool's side effects. Every effect that can
// write shares a context-aware gate owned by its server.
type Effect int

const (
	EffectRead Effect = iota
	EffectLocal
	EffectRepo
	EffectExternal
)

func annotations(e Effect) *mcp.ToolAnnotations {
	destructive, openWorld := e >= EffectRepo, e == EffectExternal
	return &mcp.ToolAnnotations{ReadOnlyHint: e == EffectRead, DestructiveHint: &destructive,
		IdempotentHint: e == EffectRead, OpenWorldHint: &openWorld}
}

func addTool[In, Out any](s *Server, name, descES string, e Effect, h func(context.Context, In, Progress) (Out, int, error)) {
	schema, err := outputSchema(reflect.TypeFor[Out]())
	if err != nil {
		panic(fmt.Sprintf("output schema for %s: %v", name, err))
	}
	var out Out
	if a, ok := any(out).(schemaAdjuster); ok {
		a.adjustSchema(schema)
	}
	mcp.AddTool(s.MCP, &mcp.Tool{Name: name, Description: descES, Annotations: annotations(e), OutputSchema: schema}, toolHandler(s, name, e, h))
}

// Engine reports retain the exact CLI JSON, including nil maps. The SDK's
// default inference makes maps non-nullable; describe their actual Go JSON
// representation instead of changing reports just to satisfy a schema.
func outputSchema(t reflect.Type) (*jsonschema.Schema, error) {
	maps := map[reflect.Type]*jsonschema.Schema{}
	seen := map[reflect.Type]bool{}
	var visit func(reflect.Type) error
	visit = func(t reflect.Type) error {
		if seen[t] {
			return nil
		}
		seen[t] = true
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array:
			return visit(t.Elem())
		case reflect.Struct:
			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				if f.IsExported() && f.Tag.Get("json") != "-" {
					if err := visit(f.Type); err != nil {
						return err
					}
				}
			}
		case reflect.Map:
			if err := visit(t.Elem()); err != nil {
				return err
			}
			s, err := jsonschema.ForType(t, &jsonschema.ForOptions{TypeSchemas: maps})
			if err != nil {
				return err
			}
			s.Type = ""
			s.Types = []string{"object", "null"}
			maps[t] = s
		}
		return nil
	}
	if err := visit(t); err != nil {
		return nil, err
	}
	return jsonschema.ForType(t, &jsonschema.ForOptions{TypeSchemas: maps})
}

func toolHandler[In, Out any](s *Server, name string, e Effect, h func(context.Context, In, Progress) (Out, int, error)) mcp.ToolHandlerFor[In, Out] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		var zero Out
		if e != EffectRead {
			// Check both before and after acquisition: select may choose the gate
			// when a cancellation and an available slot become ready together.
			if ctx.Err() != nil {
				return nil, zero, fmt.Errorf("cancelled; no files were changed")
			}
			select {
			case s.writeGate <- struct{}{}:
				defer func() { <-s.writeGate }()
			case <-ctx.Done():
				return nil, zero, fmt.Errorf("cancelled; no files were changed")
			}
			if ctx.Err() != nil {
				return nil, zero, fmt.Errorf("cancelled; no files were changed")
			}
		}
		out, exit, err := h(ctx, in, progressFor(ctx, req))
		if err != nil {
			return nil, zero, err
		}
		if exit >= 2 {
			return nil, zero, fmt.Errorf("%s could not complete (exit %d)", name, exit)
		}
		return &mcp.CallToolResult{
			Meta:    mcp.Meta{"tija/exitCode": exit},
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("%s (exit %d)", summarize(name, out), exit)}},
		}, out, nil
	}
}

func summarize(name string, _ any) string { return name + " completed" }

// schemaAdjuster lets an output type correct the schema inferred from its Go
// type, for shapes the inference cannot express (an embedded report that is
// only present in some results).
type schemaAdjuster interface {
	adjustSchema(*jsonschema.Schema)
}
