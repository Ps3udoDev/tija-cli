package quality_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/quality"
)

func TestParseTRXSumsFiles(t *testing.T) {
	dir := filepath.Join("testdata", "trx")
	counts, err := quality.ParseTRX(dir)
	if err != nil {
		t.Fatalf("ParseTRX err: %v", err)
	}
	if counts == nil {
		t.Fatal("expected non-nil TestCounts")
	}
	want := &quality.TestCounts{Total: 5, Passed: 4, Failed: 1, Skipped: 0}
	if !reflect.DeepEqual(counts, want) {
		t.Fatalf("ParseTRX = %+v, want %+v", counts, want)
	}
}

func TestParseTRXEmptyDir(t *testing.T) {
	emptyDir := t.TempDir()
	counts, err := quality.ParseTRX(emptyDir)
	if err != nil {
		t.Fatalf("ParseTRX unexpected err: %v", err)
	}
	if counts != nil {
		t.Fatalf("expected nil for empty dir, got %+v", counts)
	}
}

func TestParseJestJSON(t *testing.T) {
	vitestPath := filepath.Join("testdata", "vitest.json")
	vCounts, err := quality.ParseJestJSON(vitestPath)
	if err != nil {
		t.Fatalf("ParseJestJSON(vitest.json) err: %v", err)
	}
	vWant := &quality.TestCounts{Total: 12, Passed: 11, Failed: 0, Skipped: 1}
	if !reflect.DeepEqual(vCounts, vWant) {
		t.Fatalf("vitest counts = %+v, want %+v", vCounts, vWant)
	}

	jestPath := filepath.Join("testdata", "jest-failed.json")
	jCounts, err := quality.ParseJestJSON(jestPath)
	if err != nil {
		t.Fatalf("ParseJestJSON(jest-failed.json) err: %v", err)
	}
	if jCounts.Failed != 2 {
		t.Fatalf("jest Failed = %d, want 2", jCounts.Failed)
	}
}

func TestParseReportMalformed(t *testing.T) {
	f := filepath.Join(t.TempDir(), "malformed.json")
	if err := os.WriteFile(f, []byte("invalid json {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := quality.ParseReport("vitest", f)
	if err == nil {
		t.Fatal("expected error for malformed json, got nil")
	}
}
