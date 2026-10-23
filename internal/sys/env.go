// Package sys abstracts process execution and environment access so that
// callers can be tested without touching the real system.
package sys

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Runner runs a command in dir and returns its stdout, stderr and exit code.
// err is non-nil only when the process could not be started; a non-zero
// exit code is not an error.
type Runner func(ctx context.Context, dir, name string, args ...string) (stdout, stderr string, exitCode int, err error)

// Streamer runs name in dir writing stdout and stderr, interleaved, to out.
// Same exit-code contract as Runner. A cancelled ctx kills the process.
type Streamer func(ctx context.Context, dir string, out io.Writer, name string, args ...string) (exitCode int, err error)

// InputRunner is a Runner that also writes stdin to the process and closes it.
type InputRunner func(ctx context.Context, dir, stdin, name string, args ...string) (stdout, stderr string, exitCode int, err error)

// Env bundles the system capabilities Tija depends on.
type Env struct {
	Run      Runner
	Stream   Streamer
	RunInput InputRunner
	LookPath func(string) (string, error)
	Getenv   func(string) string
}

// Default returns an Env backed by os/exec and the os package.
func Default() Env {
	return Env{Run: execRun, Stream: execStream, RunInput: execRunInput, LookPath: exec.LookPath, Getenv: os.Getenv}
}

func execRun(ctx context.Context, dir, name string, args ...string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	if err == nil {
		return out.String(), errOut.String(), 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return out.String(), errOut.String(), exitErr.ExitCode(), nil
	}
	return out.String(), errOut.String(), -1, err
}

func execStream(ctx context.Context, dir string, out io.Writer, name string, args ...string) (int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.WaitDelay = 2 * time.Second
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return -1, err
}

func execRunInput(ctx context.Context, dir, stdin, name string, args ...string) (string, string, int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(stdin)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	cmd.WaitDelay = 2 * time.Second
	err := cmd.Run()
	if err == nil {
		return out.String(), errOut.String(), 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return out.String(), errOut.String(), exitErr.ExitCode(), nil
	}
	return out.String(), errOut.String(), -1, err
}
