package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"github.com/Ps3udoDev/tija-cli/internal/detect"
	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

type QualityRequest struct {
	Kinds   []quality.Kind
	Mode    quality.Mode
	Timeout time.Duration
	Verbose io.Writer
}

type QualityEventType string

const (
	EventStart QualityEventType = "start"
	EventDone  QualityEventType = "done"
)

type QualityEvent struct {
	Type  QualityEventType
	Index int
	Total int
	Step  quality.StepResult
}

type QualityReport struct {
	Root         string               `json:"root"`
	Initialized  bool                 `json:"initialized"`
	Mode         string               `json:"mode"`
	Commit       string               `json:"commit"`
	Dirty        bool                 `json:"dirty"`
	Steps        []quality.StepResult `json:"steps"`
	ChangedFiles []string             `json:"changedFiles"`
	Warnings     []string             `json:"warnings"`
}

func (r QualityReport) ExitCode() int {
	for _, s := range r.Steps {
		if s.Status == quality.StatusFailed {
			return 1
		}
	}
	for _, s := range r.Steps {
		if s.Status == quality.StatusUnavailable {
			return 2
		}
	}
	if len(r.Steps) == 0 {
		return 2
	}
	return 0
}

func RunQuality(ctx context.Context, dir string, env sys.Env, req QualityRequest, onEvent func(QualityEvent)) (QualityReport, error) {
	root, ok, err := gitx.FindRoot(ctx, env.Run, dir)
	if err != nil {
		return QualityReport{}, err
	}
	if !ok {
		return QualityReport{}, errors.New("not a git repository")
	}

	repo := gitx.Repo{Dir: root, Run: env.Run}

	var warnings []string
	initialized := true
	var validManifest *manifest.Manifest

	m, err := manifest.Load(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			initialized = false
			warnings = append(warnings, "not initialized — results are not saved (run: tija init)")
		} else {
			warnings = append(warnings, "manifest invalid — using detected tools")
		}
	} else {
		if errs := m.Validate(); len(errs) > 0 {
			warnings = append(warnings, "manifest invalid — using detected tools")
		} else {
			validManifest = &m
		}
	}

	remoteURL := repo.RemoteURL(ctx)
	d, err := detect.Detect(root, remoteURL)
	if err != nil {
		return QualityReport{}, err
	}
	if validManifest != nil && validManifest.Project.Solution != "" {
		d, _ = d.WithSolution(validManifest.Project.Solution)
	}

	commit, _ := repo.Head(ctx)
	staged, unstaged, untracked := repo.WorkTree(ctx)
	dirty := (staged + unstaged + untracked) > 0

	var beforeHashes map[string]string
	if req.Mode == quality.FixMode {
		beforePaths := repo.StatusPaths(ctx)
		beforeHashes = hashFiles(root, beforePaths)
	}

	var tmpDir string
	if initialized {
		tmpDir = filepath.Join(root, ".tija", "tmp", "quality")
		_ = os.MkdirAll(tmpDir, 0o755)
	} else {
		tmp, err := os.MkdirTemp("", "tija-quality-*")
		if err != nil {
			return QualityReport{}, err
		}
		defer os.RemoveAll(tmp)
		tmpDir = tmp
	}

	project := quality.Project{
		Root:           root,
		PackageManager: d.PackageManager,
		Solution:       d.Solution,
		Scripts:        d.Scripts,
		TmpDir:         tmpDir,
		GOOS:           "",
	}

	steps := quality.Resolve(validManifest, d, req.Kinds)
	invocations := quality.Plan(project, steps, req.Mode, env.LookPath)

	stepTimeout := req.Timeout
	if stepTimeout <= 0 {
		stepTimeout = 10 * time.Minute
	}

	var stepResults []quality.StepResult
	failedBuild := false
	timedOut := false

	for i, inv := range invocations {
		result := quality.StepResult{
			Kinds:    inv.Kinds,
			Provider: inv.Provider,
			Command:  inv.Display(),
			Commit:   commit,
			Dirty:    dirty,
		}

		if failedBuild || timedOut {
			result.Status = quality.StatusSkipped
			stepResults = append(stepResults, result)
			continue
		}

		if inv.Path == "" {
			result.Status = quality.StatusUnavailable
			result.Hint = inv.Hint
			stepResults = append(stepResults, result)
			continue
		}

		if inv.Report != "" {
			_ = os.RemoveAll(tmpDir)
			_ = os.MkdirAll(tmpDir, 0o755)
		}

		var writers []io.Writer
		tail := &sys.TailWriter{Max: 40}
		writers = append(writers, tail)
		if req.Verbose != nil {
			writers = append(writers, req.Verbose)
		}

		var logFile *os.File
		if initialized && len(inv.Kinds) > 0 {
			firstKind := string(inv.Kinds[0])
			logRel := filepath.ToSlash(filepath.Join(".tija", "session", "logs", firstKind+".log"))
			logAbs := filepath.Join(root, filepath.FromSlash(logRel))
			_ = os.MkdirAll(filepath.Dir(logAbs), 0o755)
			f, err := os.Create(logAbs)
			if err == nil {
				logFile = f
				writers = append(writers, f)
				result.Log = logRel
			}
		}

		outWriter := io.MultiWriter(writers...)

		if onEvent != nil {
			onEvent(QualityEvent{
				Type:  EventStart,
				Index: i,
				Total: len(invocations),
				Step:  result,
			})
		}

		stepCtx, cancel := context.WithTimeout(ctx, stepTimeout)
		start := time.Now()
		exitCode, runErr := env.Stream(stepCtx, root, outWriter, inv.Path, inv.Args...)
		duration := time.Since(start)
		isDeadline := errors.Is(stepCtx.Err(), context.DeadlineExceeded)
		cancel()

		if logFile != nil {
			_ = logFile.Close()
		}

		result.ExitCode = exitCode
		result.DurationMs = duration.Milliseconds()
		result.StartedAt = start
		result.Tail = tail.Lines()

		if isDeadline {
			timedOut = true
			result.Status = quality.StatusFailed
			result.TimedOut = true
		} else if exitCode != 0 || runErr != nil {
			result.Status = quality.StatusFailed
			if slices.Contains(inv.Kinds, quality.Build) {
				failedBuild = true
			}
		} else {
			result.Status = quality.StatusPassed
		}

		if inv.Report != "" {
			counts, err := quality.ParseReport(inv.Provider, inv.Report)
			if err == nil && counts != nil {
				result.Tests = counts
			}
		}

		if onEvent != nil {
			onEvent(QualityEvent{
				Type:  EventDone,
				Index: i,
				Total: len(invocations),
				Step:  result,
			})
		}

		stepResults = append(stepResults, result)
	}

	modeStr := "check"
	if req.Mode == quality.FixMode {
		modeStr = "fix"
	}

	changedFiles := make([]string, 0)
	if req.Mode == quality.FixMode {
		afterPaths := repo.StatusPaths(ctx)
		afterHashes := hashFiles(root, afterPaths)
		for p, h := range afterHashes {
			if oldH, ok := beforeHashes[p]; !ok || oldH != h {
				changedFiles = append(changedFiles, p)
			}
		}
		sort.Strings(changedFiles)
	} else if req.Mode == quality.CheckMode && initialized {
		_ = quality.SaveSession(root, stepResults)
	}

	if stepResults == nil {
		stepResults = make([]quality.StepResult, 0)
	}
	if warnings == nil {
		warnings = make([]string, 0)
	}

	return QualityReport{
		Root:         root,
		Initialized:  initialized,
		Mode:         modeStr,
		Commit:       commit,
		Dirty:        dirty,
		Steps:        stepResults,
		ChangedFiles: changedFiles,
		Warnings:     warnings,
	}, nil
}

func hashFiles(root string, paths []string) map[string]string {
	hashes := make(map[string]string, len(paths))
	for _, p := range paths {
		abs := filepath.Join(root, filepath.FromSlash(p))
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		sum := sha256.Sum256(data)
		hashes[p] = hex.EncodeToString(sum[:])
	}
	return hashes
}
