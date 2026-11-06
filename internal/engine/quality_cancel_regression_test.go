package engine_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/engine"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

func TestCancelledChangesShouldBeReported(t *testing.T) {
	for _, mode := range []string{"deletion", "restore-to-HEAD"} {
		t.Run(mode, func(t *testing.T) {
			fixture, err := filepath.Abs(filepath.Join("..", "detect", "testdata", "react-vite"))
			if err != nil {
				t.Fatal(err)
			}
			root := testutil.NewRepo(t, fixture)
			bin := filepath.Join(root, "node_modules", ".bin")
			if err := os.MkdirAll(bin, 0755); err != nil {
				t.Fatal(err)
			}
			for _, ext := range []string{"", ".cmd", ".exe"} {
				if err := os.WriteFile(filepath.Join(bin, "biome"+ext), []byte("fake"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(root, "victim.ts")
			if err := os.WriteFile(path, []byte("original\n"), 0644); err != nil {
				t.Fatal(err)
			}
			testutil.Git(t, root, "add", "victim.ts")
			testutil.Git(t, root, "commit", "-m", "test: victim")
			if mode == "restore-to-HEAD" {
				if err := os.WriteFile(path, []byte("dirty before formatter\n"), 0644); err != nil {
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
				return "/fake/" + name, nil
			}
			env.Stream = func(context.Context, string, io.Writer, string, ...string) (int, error) {
				var err error
				if mode == "deletion" {
					err = os.Remove(path)
				} else {
					err = os.WriteFile(path, []byte("original\n"), 0644)
				}
				if err != nil {
					t.Fatal(err)
				}
				cancel()
				return -1, ctx.Err()
			}
			r, err := engine.RunQuality(ctx, root, env, engine.QualityRequest{Mode: quality.FixMode, Kinds: []quality.Kind{quality.Format}}, nil)
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("not cancelled: %v", err)
			}
			if !slices.Contains(r.ChangedFiles, "victim.ts") {
				t.Errorf("formatter %s changed victim.ts, but ChangedFiles=%v", mode, r.ChangedFiles)
			}
		})
	}
}
