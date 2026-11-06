package mcpserver

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

func TestQualityFixCancelReportsChangedFiles(t *testing.T) {
	root := testutil.NewRepo(t, filepath.Join("..", "detect", "testdata", "react-vite"))
	bin := filepath.Join(root, "node_modules", ".bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{"", ".cmd", ".exe"} {
		if err := os.WriteFile(filepath.Join(bin, "biome"+ext), []byte("echo ok"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env := sys.Default()
	look := env.LookPath
	env.LookPath = func(name string) (string, error) {
		if name == "git" {
			return look(name)
		}
		return "/bin/" + name, nil
	}
	env.Stream = func(ctx context.Context, dir string, _ io.Writer, _ string, _ ...string) (int, error) {
		// The formatter rewrites a file, then hangs until the call is cancelled.
		_ = os.MkdirAll(filepath.Join(dir, "src"), 0o755)
		_ = os.WriteFile(filepath.Join(dir, "src", "a.ts"), []byte("formatted"), 0o644)
		cancel()
		<-ctx.Done()
		return -1, ctx.Err()
	}

	s := New(root, Deps{Env: env})
	_, _, err := s.qualityFix(ctx, qualityFixInput{Write: true}, func(string, int, int) {})
	if err == nil || err.Error() != "cancelled; changed files: src/a.ts" {
		t.Fatalf("err = %v, want cancelled; changed files: src/a.ts", err)
	}
}

func TestCancelledErrorWithoutChanges(t *testing.T) {
	if got := cancelledError(nil).Error(); got != "cancelled; no files were changed" {
		t.Errorf("cancelledError(nil) = %q", got)
	}
}
