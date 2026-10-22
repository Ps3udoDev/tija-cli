// Package testutil provides helpers for tests that need real git repositories.
package testutil

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type repoConfig struct {
	noCommit bool
	branch   string
	remote   bool
	bareDir  string
	setHead  bool
}

// RepoOpt customizes NewRepo.
type RepoOpt func(*repoConfig)

// WithoutCommit leaves the repository without any commit.
func WithoutCommit() RepoOpt { return func(c *repoConfig) { c.noCommit = true } }

// WithBranch checks out a new branch with the given name after the initial commit.
func WithBranch(name string) RepoOpt { return func(c *repoConfig) { c.branch = name } }

// WithRemote adds an origin remote backed by a bare repository (created in a
// temp dir when bareDir is empty), pushes main and, when setHead is true,
// points refs/remotes/origin/HEAD at origin/main.
func WithRemote(bareDir string, setHead bool) RepoOpt {
	return func(c *repoConfig) { c.remote, c.bareDir, c.setHead = true, bareDir, setHead }
}

// hermeticEnv isolates git from the global and system config of the machine.
func hermeticEnv(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	t.Setenv("GIT_AUTHOR_NAME", "Tija Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@tija.dev")
	t.Setenv("GIT_COMMITTER_NAME", "Tija Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@tija.dev")
}

// Git runs git in dir and fails the test on error. It returns trimmed stdout.
func Git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "core.autocrlf=false"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// NewRepo creates a git repository under t.TempDir()/"azure 2" (a path with a
// space on purpose), optionally seeded from fixtureDir, and returns its path.
func NewRepo(t *testing.T, fixtureDir string, opts ...RepoOpt) string {
	t.Helper()
	var cfg repoConfig
	for _, o := range opts {
		o(&cfg)
	}
	hermeticEnv(t)

	name := "repo"
	if fixtureDir != "" {
		name = filepath.Base(fixtureDir)
	}
	dir := filepath.Join(t.TempDir(), "azure 2", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if fixtureDir != "" {
		copyDir(t, fixtureDir, dir)
	}

	Git(t, dir, "init", "-b", "main")
	if !cfg.noCommit {
		if fixtureDir == "" {
			if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		Git(t, dir, "add", "-A")
		Git(t, dir, "commit", "--allow-empty", "-m", "initial commit")
	}
	if cfg.remote {
		bare := cfg.bareDir
		if bare == "" {
			bare = t.TempDir()
		}
		Git(t, bare, "init", "--bare", "-b", "main")
		Git(t, dir, "remote", "add", "origin", bare)
		if !cfg.noCommit {
			Git(t, dir, "push", "origin", "main")
			if cfg.setHead {
				Git(t, dir, "remote", "set-head", "origin", "main")
			}
		}
	}
	if cfg.branch != "" && cfg.branch != "main" {
		if cfg.noCommit {
			Git(t, dir, "symbolic-ref", "HEAD", "refs/heads/"+cfg.branch)
		} else {
			Git(t, dir, "checkout", "-b", cfg.branch)
		}
	}
	return dir
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy fixture %s: %v", src, err)
	}
}
