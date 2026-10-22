package engine

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/agents"
	"github.com/Ps3udoDev/tija-cli/internal/detect"
	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

// CheckStatus is the outcome of one doctor check.
type CheckStatus string

const (
	StatusOK   CheckStatus = "ok"
	StatusWarn CheckStatus = "warn"
	StatusFail CheckStatus = "fail"
)

// Check is one doctor finding.
type Check struct {
	ID     string      `json:"id"`
	Title  string      `json:"title"`
	Status CheckStatus `json:"status"`
	Hint   string      `json:"hint,omitempty"`
}

// DoctorReport is the result of Doctor.
type DoctorReport struct {
	Root         string   `json:"root"`
	Checks       []Check  `json:"checks"`
	Agents       []string `json:"agents"`
	DefaultAgent string   `json:"defaultAgent"`
	// Detection is what doctor detected in the repository; nil outside one.
	Detection *detect.Detection `json:"detection,omitempty"`
}

// ExitCode is 1 when any check failed, else 0.
func (r DoctorReport) ExitCode() int {
	for _, c := range r.Checks {
		if c.Status == StatusFail {
			return 1
		}
	}
	return 0
}

// nodeTools are looked up in node_modules/.bin before PATH.
var nodeTools = []string{"biome", "eslint", "prettier", "vitest", "jest", "playwright"}

// Doctor inspects the environment and the repository containing dir. It
// never writes anything.
func Doctor(ctx context.Context, dir string, env sys.Env) (DoctorReport, error) {
	rep := DoctorReport{Checks: []Check{}, Agents: []string{}}
	// add records a check that is ok when ok is true and failStatus otherwise.
	add := func(id, title string, ok bool, failStatus CheckStatus, hint string) {
		c := Check{ID: id, Title: title, Status: StatusOK}
		if !ok {
			c.Status, c.Hint = failStatus, hint
		}
		rep.Checks = append(rep.Checks, c)
	}
	has := func(name string) bool {
		p, err := env.LookPath(name)
		return err == nil && p != ""
	}

	add("git.installed", "Git installed", has("git"), StatusFail, "install git")

	root, inRepo, err := gitx.FindRoot(ctx, env.Run, dir)
	if err != nil {
		inRepo = false // e.g. git cannot be started; reported above
	}
	add("repo.detected", "repository detected", inRepo, StatusFail, "run inside a git repository")

	var (
		d       detect.Detection
		m       manifest.Manifest
		mOK     bool
		haveDet bool
	)
	if inRepo {
		rep.Root = root
		repo := gitx.Repo{Dir: root, Run: env.Run}
		if d, err = detect.Detect(root, repo.RemoteURL(ctx)); err != nil {
			return rep, err
		}
		haveDet = true
		rep.Detection = &d

		data, rerr := os.ReadFile(abs(root, manifest.Path))
		present := rerr == nil
		add("manifest.present", "manifest present", present, StatusFail, "run: tija init")
		if present {
			var problem string
			m, problem = parseValid(data)
			mOK = problem == ""
			c := Check{ID: "manifest.valid", Title: "manifest valid", Status: StatusOK}
			if !mOK {
				c.Status, c.Title = StatusFail, "manifest invalid: "+problem
			}
			rep.Checks = append(rep.Checks, c)
		}
	}

	if mOK {
		for _, tool := range toolChecks(m, d) {
			add("tool."+tool.id, tool.id+" available", toolFound(root, tool.bin, has), StatusFail, "install "+tool.bin)
		}
	}

	ghOK := has("gh")
	add("gh.installed", "GitHub CLI installed", ghOK, StatusFail, "install GitHub CLI: https://cli.github.com")
	if ghOK {
		_, code, rerr := env.Run(ctx, dir, "gh", "auth", "status")
		add("gh.auth", "GitHub CLI authenticated", rerr == nil && code == 0, StatusFail, "run: gh auth login")
	}

	if found := agents.Available(env.LookPath); len(found) > 0 {
		rep.Agents = found
		rep.DefaultAgent = found[0]
		if mOK && slices.Contains(found, m.Agents.Default) {
			rep.DefaultAgent = m.Agents.Default
		}
		rep.Checks = append(rep.Checks, Check{ID: "agents.available", Status: StatusOK,
			Title: "agents: " + strings.Join(found, ", ") + " (default: " + rep.DefaultAgent + ")"})
	} else {
		rep.Checks = append(rep.Checks, Check{ID: "agents.available", Status: StatusWarn,
			Title: "no agent CLI found", Hint: "install claude, codex or agy/gemini"})
	}

	if mOK {
		_, serr := os.Stat(abs(root, m.PullRequest.Template))
		add("pr.template", "PR template found", serr == nil, StatusFail, "run: tija init")
	}
	if haveDet {
		if d.PRGovernanceWorkflow != "" {
			rep.Checks = append(rep.Checks, Check{ID: "pr.governance", Status: StatusWarn,
				Title: "verify manifest rules match " + d.PRGovernanceWorkflow})
		}
		if len(d.Lockfiles) > 1 {
			rep.Checks = append(rep.Checks, Check{ID: "lockfiles.multiple", Status: StatusWarn,
				Title: "multiple lockfiles: " + strings.Join(d.Lockfiles, ", "), Hint: "keep a single lockfile"})
		}
		if d.TestCount == 0 {
			rep.Checks = append(rep.Checks, Check{ID: "tests.none", Status: StatusWarn, Title: "no tests detected"})
		}
	}
	return rep, nil
}

type toolCheck struct{ id, bin string }

// toolChecks lists the distinct quality providers of m plus the package
// manager binary, in a stable order.
func toolChecks(m manifest.Manifest, d detect.Detection) []toolCheck {
	var ids []string
	for _, q := range m.Quality {
		if q.Provider != "" {
			ids = append(ids, q.Provider)
		}
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if pm := d.PackageManager; pm != "" && !slices.Contains(ids, pm) {
		ids = append(ids, pm)
	}
	out := make([]toolCheck, len(ids))
	for i, id := range ids {
		bin := id
		if strings.HasPrefix(id, "dotnet-") {
			bin = "dotnet"
		}
		out[i] = toolCheck{id, bin}
	}
	return out
}

func toolFound(root, bin string, has func(string) bool) bool {
	if slices.Contains(nodeTools, bin) {
		names := []string{bin}
		if runtime.GOOS == "windows" {
			names = append(names, bin+".cmd", bin+".exe")
		}
		for _, n := range names {
			if st, err := os.Stat(filepath.Join(root, "node_modules", ".bin", n)); err == nil && !st.IsDir() {
				return true
			}
		}
	}
	return has(bin)
}
