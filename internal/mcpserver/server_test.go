package mcpserver

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestResolveRootFixesGitRoot(t *testing.T) {
	root := testutil.NewRepo(t, "")
	sub := filepath.Join(root, "nested", "directory")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveRoot(context.Background(), sys.Default(), sub)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(root) {
		t.Fatalf("root = %q, want %q", got, root)
	}
	outside := t.TempDir()
	if _, err := ResolveRoot(context.Background(), sys.Default(), outside); !errors.Is(err, engine.ErrNotARepo) || !strings.Contains(err.Error(), "is not a git repository") {
		t.Fatalf("outside repository error = %v", err)
	}
}

func TestResolveRootUsesWorkingDirectory(t *testing.T) {
	root := testutil.NewRepo(t, "")
	t.Chdir(root)
	got, err := ResolveRoot(context.Background(), sys.Default(), "")
	if err != nil || filepath.Clean(got) != filepath.Clean(root) {
		t.Fatalf("root = %q, error = %v, want %q", got, err, root)
	}
}

func TestInitializeReportsTijaAndInstructions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s := New(t.TempDir(), Deps{Env: sys.Default(), Version: "test-version"})
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, serverTransport) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	init := session.InitializeResult()
	if init.ServerInfo.Name != "tija" || init.ServerInfo.Version != "test-version" {
		t.Fatalf("server info = %#v", init.ServerInfo)
	}
	for _, want := range []string{"raíz", "write", "datos", "órdenes"} {
		if !strings.Contains(init.Instructions, want) {
			t.Errorf("instructions missing %q: %s", want, init.Instructions)
		}
	}
	if init.Capabilities.Tools == nil || init.Capabilities.Prompts == nil || init.Capabilities.Resources != nil {
		t.Fatalf("unexpected capabilities: %#v", init.Capabilities)
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve after disconnect: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("serve did not stop after disconnect")
	}
}
