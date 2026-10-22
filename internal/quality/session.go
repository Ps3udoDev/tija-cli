package quality

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

type StepStatus string

const (
	StatusPassed      StepStatus = "passed"
	StatusFailed      StepStatus = "failed"
	StatusSkipped     StepStatus = "skipped"
	StatusUnavailable StepStatus = "unavailable"
)

type StepResult struct {
	Kinds      []Kind      `json:"kinds"`
	Provider   string      `json:"provider"`
	Command    string      `json:"command"`
	Status     StepStatus  `json:"status"`
	ExitCode   int         `json:"exitCode"`
	TimedOut   bool        `json:"timedOut,omitempty"`
	DurationMs int64       `json:"durationMs"`
	StartedAt  time.Time   `json:"startedAt"`
	Commit     string      `json:"commit"`
	Dirty      bool        `json:"dirty"`
	Tests      *TestCounts `json:"tests,omitempty"`
	Log        string      `json:"log,omitempty"` // relative to repo root, with '/'
	Hint       string      `json:"hint,omitempty"`
	Tail       []string    `json:"-"`
}

// Session is .tija/session/quality.json: the latest result per kind.
type Session struct {
	Version int                 `json:"version"`
	Results map[Kind]StepResult `json:"results"`
}

const SessionPath = ".tija/session/quality.json"

// LoadSession reads .tija/session/quality.json. Missing file yields Session{Version:1, Results:{}} and nil.
func LoadSession(root string) (Session, error) {
	path := filepath.Join(root, filepath.FromSlash(SessionPath))
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Session{Version: 1, Results: make(map[Kind]StepResult)}, nil
		}
		return Session{}, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return Session{}, err
	}
	if s.Results == nil {
		s.Results = make(map[Kind]StepResult)
	}
	if s.Version == 0 {
		s.Version = 1
	}
	return s, nil
}

// SaveSession merges results by kind into existing session and writes to .tija/session/quality.json.
func SaveSession(root string, results []StepResult) error {
	session, err := LoadSession(root)
	if err != nil {
		return err
	}
	for _, res := range results {
		for _, k := range res.Kinds {
			session.Results[k] = res
		}
	}
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	target := filepath.Join(root, filepath.FromSlash(SessionPath))
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(dir, "quality-session-*.json")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}

	_ = os.Remove(target)
	if err := os.Rename(tmpName, target); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}
