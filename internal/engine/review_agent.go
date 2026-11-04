package engine

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/agents"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/review"
	"github.com/Ps3udoDev/tija-cli/internal/rules"
	"github.com/Ps3udoDev/tija-cli/internal/skills"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"golang.org/x/sync/errgroup"
)

func reviewAgents(ctx context.Context, root string, env sys.Env, req ReviewRequest, m manifest.Manifest, name string, chunks [][]review.FileDiff, packs map[string]skills.Pack, set rules.Set, quality []string, rep *ReviewReport, emit func(ReviewEvent)) {
	passes := []string{req.Mode}
	if req.Mode == "deep" {
		passes = []string{"review", "quality"}
	}
	tier := m.Agents.Tasks["review"].Tier
	if tier == "" {
		tier = "balanced"
	}
	if req.Mode == "deep" {
		tier = "deep"
	}
	timeout := req.AgentTimeout
	if timeout == 0 {
		timeout = map[string]time.Duration{"fast": 3 * time.Minute, "balanced": 6 * time.Minute, "deep": 12 * time.Minute}[tier]
		if timeout == 0 {
			timeout = 6 * time.Minute
		}
	}
	type outcome struct {
		findings  []review.Finding
		discarded []review.Discarded
		err       error
	}
	results := make([][]outcome, len(chunks))
	for i := range results {
		results[i] = make([]outcome, len(passes))
	}
	var group errgroup.Group
	group.SetLimit(3)
	var mu sync.Mutex
	for i, chunk := range chunks {
		for j, pass := range passes {
			group.Go(func() error {
				var result outcome
				defer func() { mu.Lock(); results[i][j] = result; mu.Unlock() }()
				paths := make([]string, 0, len(chunk))
				files := map[string]review.FileDiff{}
				for _, f := range chunk {
					paths = append(paths, f.Path)
					files[f.Path] = f
				}
				pack := packs[pass]
				allowed := map[string]bool{}
				for _, id := range pack.Rules {
					if id != "1000-lines-ceiling" {
						allowed[id] = true
					}
				}
				prules := []review.PromptRule{}
				for _, r := range set.AgentRules(paths) {
					allowed[r.ID] = true
					prules = append(prules, review.PromptRule{ID: r.ID, Title: r.Title, Text: r.Text})
				}
				handled := []string{"1000-lines-ceiling"}
				for _, c := range set.Checks() {
					handled = append(handled, c.ID)
				}
				input := review.PromptInput{PackBody: pack.Body, Rules: prules, Quality: quality, Files: chunk, Handled: handled, Language: m.Content.Language}
				// Separate temporary output per call prevents concurrent Codex
				// invocations from overwriting one another's last-message file.
				tmp, err := os.MkdirTemp("", "tija-review-")
				if err != nil {
					result.err = err
					return nil
				}
				defer os.RemoveAll(tmp)
				for attempt := 0; attempt < 2; attempt++ {
					emit(ReviewEvent{Stage: StageAgent, Message: fmt.Sprintf("%s: chunk %d/%d (attempt %d)", pass, i+1, len(chunks), attempt+1), Pass: pass, Chunk: i + 1, Chunks: len(chunks)})
					out, err := agents.Generate(ctx, env, name, agents.Request{Prompt: review.BuildPrompt(input), Tier: tier, Dir: root, TmpDir: tmp, Timeout: timeout, ReadOnly: true})
					var raw []review.AgentFinding
					if err == nil {
						raw, err = review.ParseResponse(out)
					}
					if err != nil {
						result.err = err
						input.Retry = oneLine(err)
						if ctx.Err() != nil {
							break
						}
						continue
					}
					emit(ReviewEvent{Stage: StageVerify, Message: "verifying agent findings", Pass: pass, Chunk: i + 1, Chunks: len(chunks)})
					result.findings, result.discarded = review.Verify(review.VerifyInput{Pass: pass, Files: files, ReadLines: func(p string) ([]string, error) { return reviewReadLines(root, p) }, Rules: allowed}, raw)
					result.err = nil
					break
				}
				return nil
			})
		}
	}
	_ = group.Wait()
	for i, row := range results {
		failed := false
		for j, r := range row {
			if r.err != nil {
				failed = true
				rep.Warnings = append(rep.Warnings, fmt.Sprintf("%s chunk %d failed: %s", passes[j], i+1, oneLine(r.err)))
			}
			rep.Findings = append(rep.Findings, r.findings...)
			rep.Discarded = append(rep.Discarded, r.discarded...)
		}
		if failed {
			rep.Chunks.Failed++
		}
	}
}
