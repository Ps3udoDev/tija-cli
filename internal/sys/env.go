// Package sys abstracts process execution and environment access so that
// callers can be tested without touching the real system.
package sys

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
)

// Runner runs a command in dir and returns its stdout and exit code.
// err is non-nil only when the process could not be started; a non-zero
// exit code is not an error.
type Runner func(ctx context.Context, dir, name string, args ...string) (stdout string, exitCode int, err error)

// Env bundles the system capabilities Tija depends on.
type Env struct {
	Run      Runner
	LookPath func(string) (string, error)
	Getenv   func(string) string
}

// Default returns an Env backed by os/exec and the os package.
func Default() Env {
	return Env{Run: execRun, LookPath: exec.LookPath, Getenv: os.Getenv}
}

func execRun(ctx context.Context, dir, name string, args ...string) (string, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err == nil {
		return out.String(), 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return out.String(), exitErr.ExitCode(), nil
	}
	return out.String(), -1, err
}
