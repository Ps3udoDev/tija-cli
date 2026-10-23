package quality_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/quality"
)

func TestLookupBinWindowsShim(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "node_modules", ".bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cmdPath := filepath.Join(binDir, "biome.cmd")
	if err := os.WriteFile(cmdPath, []byte("echo biome"), 0o755); err != nil {
		t.Fatal(err)
	}
	shPath := filepath.Join(binDir, "biome")
	if err := os.WriteFile(shPath, []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}

	fakeLookPath := func(name string) (string, error) {
		return "", errors.New("not found")
	}

	// Windows should find biome.cmd
	gotWin, ok := quality.LookupBin(root, "biome", "windows", fakeLookPath)
	if !ok || gotWin != cmdPath {
		t.Fatalf("windows: got %q, %v; want %q, true", gotWin, ok, cmdPath)
	}

	// Linux should find biome
	gotLinux, ok := quality.LookupBin(root, "biome", "linux", fakeLookPath)
	if !ok || gotLinux != shPath {
		t.Fatalf("linux: got %q, %v; want %q, true", gotLinux, ok, shPath)
	}

	// Nonexistent node tool and lookPath fails -> ok=false
	_, ok = quality.LookupBin(root, "eslint", "windows", fakeLookPath)
	if ok {
		t.Fatalf("eslint: expected ok=false, got true")
	}
}

func TestLookupBinNonNodeUsesPath(t *testing.T) {
	root := t.TempDir()
	fakeLookPath := func(name string) (string, error) {
		if name == "dotnet" {
			return "/usr/bin/dotnet", nil
		}
		return "", errors.New("not found")
	}

	got, ok := quality.LookupBin(root, "dotnet", "linux", fakeLookPath)
	if !ok || got != "/usr/bin/dotnet" {
		t.Fatalf("got %q, %v; want /usr/bin/dotnet, true", got, ok)
	}
}
