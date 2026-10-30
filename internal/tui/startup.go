package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Ps3udoDev/tija-cli/internal/detect"
	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/workflows"
)

// startupInfo is what the startup screen shows about the repository.
type startupInfo struct {
	Root         string // repository root, or the working directory outside a repo
	InRepo       bool
	Detection    *detect.Detection // nil outside a repo or when detection failed
	Initialized  bool              // .tija/manifest.yaml exists at the root
	Err          string            // real error if FindRoot failed
	HasJS        bool
	HasWorkflows bool
}

// startupMsg delivers the result of loadStartup.
type startupMsg struct{ info startupInfo }

// loadStartup finds the repository containing dir and detects its stack.
func loadStartup(dir string, env sys.Env) startupInfo {
	ctx := context.Background()
	info := startupInfo{Root: dir}
	root, ok, err := gitx.FindRoot(ctx, env.Run, dir)
	if err != nil {
		info.Err = err.Error()
		return info
	}
	if !ok {
		return info
	}
	info.Root, info.InRepo = root, true
	remote := gitx.Repo{Dir: root, Run: env.Run}.RemoteURL(ctx)
	if d, err := detect.Detect(root, remote); err == nil {
		info.Detection = &d
		if d.HasPackageJSON || d.Stack == detect.StackReactVite {
			info.HasJS = true
		}
	}
	if m, err := manifest.Load(root); err == nil {
		info.Initialized = true
		if m.Project.Type == "react-vite" {
			info.HasJS = true
		}
	} else if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(manifest.Path))); err == nil {
		info.Initialized = true
	}
	if wfs, _ := workflows.Load(root); len(wfs) > 0 {
		for _, w := range wfs {
			if len(workflows.Validate(w)) == 0 {
				info.HasWorkflows = true
				break
			}
		}
	}
	return info
}

func detectCmd(dir string, env sys.Env) tea.Cmd {
	return func() tea.Msg { return startupMsg{info: loadStartup(dir, env)} }
}

const tagline = "Repository engineering for humans & agents"

// wideLayout is the minimum width for side-by-side columns.
const wideLayout = 80

// displayVersion prefixes release versions with "v" and leaves others, such as "dev", as is.
func displayVersion(version string) string {
	if version != "" && version[0] >= '0' && version[0] <= '9' {
		return "v" + version
	}
	return version
}

// topBar shows the app name and path on the left and the version on the right.
func topBar(path, version string, width int) string {
	left := logoStyle.Render("tija") + "  " + mutedStyle.Render(path)
	right := mutedStyle.Render(displayVersion(version))
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		return left + "  " + right
	}
	return left + strings.Repeat(" ", gap) + right
}

// checklist lists what detection found.
func checklist(info startupInfo) []string {
	if !info.InRepo {
		if info.Err != "" {
			return []string{warn(info.Err)}
		}
		return []string{warn("not a git repository")}
	}
	d := info.Detection
	if d == nil {
		return []string{warn("could not detect the project")}
	}
	lines := []string{ok(d.ProjectName + " detected")}
	switch {
	case d.Framework != "":
		lines = append(lines, ok(d.Framework))
	case d.Language != "":
		lines = append(lines, ok(d.Language))
	}
	if d.GitHost == "github" {
		lines = append(lines, ok("GitHub"))
	}
	switch {
	case d.TestCount > 0:
		lines = append(lines, ok(fmt.Sprintf("%d %s", d.TestCount, plural(d.TestCount, "test"))))
	case d.TestFiles > 0:
		lines = append(lines, ok(fmt.Sprintf("%d %s", d.TestFiles, plural(d.TestFiles, "test file"))))
	default:
		lines = append(lines, warn("no tests detected"))
	}
	return lines
}

func ok(msg string) string   { return okStyle.Render("✓") + " " + msg }
func warn(msg string) string { return warnStyle.Render("!") + " " + msg }

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

// startupView keeps one colored mascot below the wordmark.
func (m model) startupView() string {
	state := Idle
	if m.screen == screenDetecting {
		state = Scanning
	}
	details := []string{mutedStyle.Render(Caption(state))}
	if m.screen != screenDetecting {
		details = append(details, "")
		details = append(details, checklist(m.info)...)
	}
	left := brandHeader(m.width) + "\n" + "Repository engineering\nfor humans & agents." + "\n\n" + Sprite(state) + "\n" + strings.Join(details, "\n")
	if m.width < wideLayout {
		left = brandHeader(m.width) + "\n" + tagline + "\n\n" + lipgloss.JoinHorizontal(lipgloss.Top, Sprite(state), "  ", strings.Join(details, "\n"))
	}
	right := func(desc bool) string {
		if m.screen == screenDetecting {
			return headingStyle.Render("Getting your repository ready") + "\n\n" + mutedStyle.Render("Detecting project, tools and tests...")
		}
		menu := renderMenu(m.items, m.cursor, desc)
		if desc && m.width >= wideLayout && m.height >= 26 {
			menu = strings.ReplaceAll(menu, "\n", "\n\n")
		}
		return headingStyle.Render("What do you want to do?") + "\n\n" + menu
	}
	var layouts []string
	if m.width >= wideLayout {
		layouts = append(layouts, columns(left, right(true)), columns(left, right(false)))
	}
	layouts = append(layouts, left+"\n\n"+right(true), left+"\n\n"+right(false))
	body := layouts[len(layouts)-1]
	for _, candidate := range layouts {
		if lipgloss.Width(candidate)+len(margin) <= m.width {
			body = candidate
			break
		}
	}
	return topBar(displayPath(m.info.Root), m.version, m.width) + "\n\n" + indent(body) + "\n\n" + m.footer()
}

// columns joins left and right with a vertical separator.
func columns(left, right string) string {
	h := max(lipgloss.Height(left), lipgloss.Height(right))
	sep := separatorStyle.Render(strings.TrimSuffix(strings.Repeat("│\n", h), "\n"))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "   ", sep, "   ", right)
}

// displayPath shortens paths under the home directory to "~".
func displayPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if rel, err := filepath.Rel(home, p); err == nil && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
		if rel == "." {
			return "~"
		}
		return "~/" + filepath.ToSlash(rel)
	}
	return p
}
