package mcpserver

import (
	"context"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Progress forwards engine stages only when the caller requested notifications.
type Progress func(stage string, done, total int)

func progressFor(ctx context.Context, req *mcp.CallToolRequest) Progress {
	if req == nil || req.Params == nil || req.Session == nil || req.Params.GetProgressToken() == nil {
		return func(string, int, int) {}
	}
	token := req.Params.GetProgressToken()
	var mu sync.Mutex
	var last float64
	return func(stage string, done, total int) {
		mu.Lock()
		defer mu.Unlock()
		p := float64(done)
		if p <= last {
			p = last + 1
		}
		last = p
		limit := float64(total)
		if limit < p {
			limit = 0
		}
		_ = req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
			ProgressToken: token, Message: stage, Progress: p, Total: limit,
		})
	}
}
