package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/agents"
)

func TestCleanExplicitAgentSelection(t *testing.T) {
	root := setupCommentsRepo(t)
	env, calls := commentFakeEnv(dynamicCommentClassifications)
	look := env.LookPath
	env.LookPath = func(name string) (string, error) {
		if name == "codex" {
			return "/fake/bin/codex", nil
		}
		return look(name)
	}
	r, err := CleanComments(context.Background(), root, env, CleanRequest{All: true, Agent: "codex"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(*calls) == 0 || r.Agent.Provider != "codex" {
		t.Fatalf("agent=%s calls=%d", r.Agent.Provider, len(*calls))
	}
	for _, c := range *calls {
		if c.name != "/fake/bin/codex" {
			t.Fatalf("unexpected provider %q", c.name)
		}
	}
}

func TestCleanExplicitAgentUnavailable(t *testing.T) {
	root := setupCommentsRepo(t)
	env, calls := commentFakeEnv(nil)
	look := env.LookPath
	env.LookPath = func(name string) (string, error) {
		if name == "codex" {
			return "", errors.New("missing")
		}
		return look(name)
	}
	_, err := CleanComments(context.Background(), root, env, CleanRequest{All: true, Agent: "codex"}, nil)
	if !errors.Is(err, agents.ErrNotInstalled) || len(*calls) != 0 {
		t.Fatalf("err=%v calls=%d", err, len(*calls))
	}
}
