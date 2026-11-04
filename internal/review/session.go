package review

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

const SessionPath = ".tija/session/review.json"

func SaveSession(root string, r Result) error {
	r.SavedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	target := filepath.Join(root, filepath.FromSlash(SessionPath))
	dir := filepath.Dir(target)
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "review-session-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	// Match the quality-session replacement strategy on Windows.
	if err = os.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.Rename(name, target)
}

func LoadSession(root string) (Result, bool, error) {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(SessionPath)))
	if errors.Is(err, fs.ErrNotExist) {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, false, err
	}
	var r Result
	if err = json.Unmarshal(data, &r); err != nil {
		return Result{}, false, err
	}
	return r, true, nil
}
