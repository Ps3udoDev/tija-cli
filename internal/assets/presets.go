package assets

import (
	"bytes"
	"embed"
	"strings"
)

const ArchitectureDir = ".tija/rules/architecture"

// Presets deliberately live outside defaults so init only installs them on opt-in.
//
//go:embed presets/architecture/*.md
var presetsFS embed.FS

func Presets() []string { return []string{"dotnet-clean", "react-features"} }
func Preset(name string) ([]byte, bool) {
	if strings.ContainsAny(name, "/\\") {
		return nil, false
	}
	data, err := presetsFS.ReadFile("presets/architecture/" + name + ".md")
	if err != nil {
		return nil, false
	}
	return bytes.Clone(data), true
}
