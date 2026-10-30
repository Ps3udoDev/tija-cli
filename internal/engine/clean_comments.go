package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/Ps3udoDev/tija-cli/internal/agents"
	"github.com/Ps3udoDev/tija-cli/internal/assets"
	"github.com/Ps3udoDev/tija-cli/internal/comments"
	"github.com/Ps3udoDev/tija-cli/internal/gitx"
	"github.com/Ps3udoDev/tija-cli/internal/manifest"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
	"github.com/Ps3udoDev/tija-cli/internal/skills"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/textdiff"
)

var planEdit = comments.PlanEdit

type CleanRequest struct {
	Paths        []string
	All          bool
	Write        bool
	Unsafe       bool
	Diff         bool
	NoAgent      bool
	Force        bool
	Concurrency  int
	AgentTimeout time.Duration
}

type CleanChange struct {
	Line   int    `json:"line"`
	Action string `json:"action"` // remove | rewrite
	Class  string `json:"class"`
	Safety string `json:"safety"` // safe | unsafe
	Source string `json:"source"` // rule | agent
	Text   string `json:"text,omitempty"`
}

type CleanFile struct {
	Path    string        `json:"path"`
	Status  string        `json:"status"` // pending | written | unchanged | skipped | failed | restored
	Reason  string        `json:"reason,omitempty"`
	Changes []CleanChange `json:"changes,omitempty"`
	Diff    string        `json:"diff,omitempty"`
}

type CleanSummary struct {
	Comments     int `json:"comments"`
	Protected    int `json:"protected"`
	Keep         int `json:"keep"`
	Safe         int `json:"safe"`
	Unsafe       int `json:"unsafe"`
	Unclassified int `json:"unclassified"`
	SkippedFiles int `json:"skippedFiles"`
	FailedFiles  int `json:"failedFiles"`
}

type CleanReport struct {
	Scope struct {
		Mode  string   `json:"mode"` // diff | paths | all
		Paths []string `json:"paths,omitempty"`
		Files int      `json:"files"`
	} `json:"scope"`
	Agent struct {
		Provider string `json:"provider,omitempty"`
		Tier     string `json:"tier,omitempty"`
		Batches  int    `json:"batches"`
	} `json:"agent"`
	Summary   CleanSummary `json:"summary"`
	Files     []CleanFile  `json:"files"`
	Written   bool         `json:"written"`
	Warnings  []string     `json:"warnings,omitempty"`
	PostCheck string       `json:"postCheck,omitempty"` // "biome" | "skipped: <reason>"

	unsafe bool
}

func (r CleanReport) ExitCode() int {
	if r.Written {
		if r.Summary.FailedFiles > 0 {
			return 1
		}
		return 0
	}
	if r.unsafe {
		if r.Summary.Safe+r.Summary.Unsafe > 0 {
			return 1
		}
		return 0
	}
	if r.Summary.Safe > 0 {
		return 1
	}
	return 0
}

type CleanEvent struct {
	Type  string // extract | classify | verify | write
	Done  int
	Total int
	Path  string
}

func CleanComments(ctx context.Context, dir string, env sys.Env, req CleanRequest, onEvent func(CleanEvent)) (CleanReport, error) {
	emit := func(e CleanEvent) {
		if onEvent != nil {
			onEvent(e)
		}
	}

	root, ok, err := gitx.FindRoot(ctx, env.Run, dir)
	if err != nil {
		return CleanReport{}, err
	}
	if !ok {
		return CleanReport{}, fmt.Errorf("%w: %s", ErrNotARepo, dir)
	}

	repo := gitx.Repo{Dir: root, Run: env.Run}
	m, initialized := loadValidManifest(root)
	if !initialized {
		m.Comments = manifest.DefaultComments()
		m.Content.Language = "es"
	}
	if len(m.Comments.Include) == 0 {
		m.Comments.Include = manifest.DefaultComments().Include
	}
	if len(m.Comments.Exclude) == 0 {
		m.Comments.Exclude = manifest.DefaultComments().Exclude
	}

	var mode string
	var searchPaths []string
	if len(req.Paths) > 0 {
		mode = "paths"
		searchPaths = req.Paths
	} else if req.All {
		mode = "all"
	} else {
		mode = "diff"
	}

	// Paths are relative to the working directory; --full-name keeps git's
	// output relative to the repo root.
	lsDir := root
	if len(searchPaths) > 0 {
		lsDir = dir
	}

	// Identify untracked files
	untrackedCmd := []string{"ls-files", "-z", "--full-name", "--others", "--exclude-standard"}
	if len(searchPaths) > 0 {
		untrackedCmd = append(untrackedCmd, "--")
		untrackedCmd = append(untrackedCmd, searchPaths...)
	}
	untrackedOut, _, _, _ := env.Run(ctx, lsDir, "git", untrackedCmd...)
	untrackedSet := make(map[string]bool)
	for _, p := range strings.Split(untrackedOut, "\x00") {
		p = strings.TrimSpace(p)
		if p != "" {
			untrackedSet[filepath.ToSlash(p)] = true
		}
	}

	// Discover candidate file paths
	candidateMap := make(map[string]bool)
	if mode == "diff" {
		target := m.PullRequest.Target
		if target == "" {
			target = defaultTarget(ctx, repo)
		}
		baseRef, ok := repo.ResolveBase(ctx, target)
		if !ok {
			baseRef = target
		}
		stats := repo.DiffStat(ctx, baseRef)
		for _, s := range stats {
			candidateMap[filepath.ToSlash(s.Path)] = true
		}
		for p := range untrackedSet {
			candidateMap[p] = true
		}
	} else {
		lsCmd := []string{"ls-files", "-z", "--full-name", "--cached", "--others", "--exclude-standard"}
		if len(searchPaths) > 0 {
			lsCmd = append(lsCmd, "--")
			lsCmd = append(lsCmd, searchPaths...)
		}
		lsOut, _, _, _ := env.Run(ctx, lsDir, "git", lsCmd...)
		for _, p := range strings.Split(lsOut, "\x00") {
			p = strings.TrimSpace(p)
			if p != "" {
				candidateMap[filepath.ToSlash(p)] = true
			}
		}
	}

	// Filter candidate paths by language and manifest include/exclude
	var candidatePaths []string
	for p := range candidateMap {
		if _, ok := comments.LangFor(p); !ok {
			continue
		}
		matchedInclude := false
		for _, inc := range m.Comments.Include {
			if comments.Match(inc, p) {
				matchedInclude = true
				break
			}
		}
		if !matchedInclude {
			continue
		}
		matchedExclude := false
		for _, exc := range m.Comments.Exclude {
			if comments.Match(exc, p) {
				matchedExclude = true
				break
			}
		}
		if matchedExclude {
			continue
		}
		candidatePaths = append(candidatePaths, p)
	}
	slices.Sort(candidatePaths)

	rep := CleanReport{}
	rep.Written = req.Write
	rep.unsafe = req.Unsafe
	rep.Scope.Mode = mode
	rep.Scope.Paths = req.Paths
	rep.Scope.Files = len(candidatePaths)
	rep.Warnings = []string{}

	// Compile manifest comments.protect regexes
	var extraProtect []*regexp.Regexp
	for _, pat := range m.Comments.Protect {
		if r, err := regexp.Compile(pat); err == nil {
			extraProtect = append(extraProtect, r)
		}
	}

	type commentWithChange struct {
		comment comments.Comment
		change  CleanChange
	}

	type fileState struct {
		relPath  string
		fullPath string
		src      []byte
		lang     comments.Lang
		comments []commentWithChange
	}

	var filesToProcess []fileState
	var unclassifiedComments []comments.Comment

	// Process extraction and deterministic rules
	for i, relPath := range candidatePaths {
		emit(CleanEvent{Type: "extract", Done: i + 1, Total: len(candidatePaths), Path: relPath})

		if untrackedSet[relPath] && !req.Force {
			rep.Files = append(rep.Files, CleanFile{
				Path:   relPath,
				Status: "skipped",
				Reason: "untracked file (use --force to include)",
			})
			rep.Summary.SkippedFiles++
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("! %s: untracked file — skipped (use --force)", relPath))
			continue
		}

		fullPath := filepath.Join(root, filepath.FromSlash(relPath))
		src, err := os.ReadFile(fullPath)
		if err != nil {
			rep.Files = append(rep.Files, CleanFile{
				Path:   relPath,
				Status: "failed",
				Reason: err.Error(),
			})
			rep.Summary.FailedFiles++
			continue
		}

		lang, _ := comments.LangFor(relPath)
		extracted, err := comments.Extract(relPath, src, lang)
		if err != nil {
			if errors.Is(err, comments.ErrUncertain) {
				rep.Files = append(rep.Files, CleanFile{
					Path:   relPath,
					Status: "skipped",
					Reason: "could not tokenize with certainty",
				})
				rep.Summary.SkippedFiles++
			} else {
				rep.Files = append(rep.Files, CleanFile{
					Path:   relPath,
					Status: "failed",
					Reason: err.Error(),
				})
				rep.Summary.FailedFiles++
			}
			continue
		}

		fState := fileState{
			relPath:  relPath,
			fullPath: fullPath,
			src:      src,
			lang:     lang,
		}

		for _, c := range extracted {
			rep.Summary.Comments++

			if comments.Protected(c, filepath.Ext(relPath), extraProtect) {
				rep.Summary.Protected++
				continue
			}

			if verdict, ok := comments.Rule(c, src); ok {
				rep.Summary.Safe++
				ch := CleanChange{
					Line:   c.Line,
					Action: verdict.Action,
					Class:  verdict.Class,
					Safety: "safe",
					Source: "rule",
				}
				fState.comments = append(fState.comments, commentWithChange{
					comment: c,
					change:  ch,
				})
				continue
			}

			unclassifiedComments = append(unclassifiedComments, c)
		}

		filesToProcess = append(filesToProcess, fState)
	}

	// Agent classification
	agentClassifications := make(map[string]comments.Classification)
	avail := agents.Available(env.LookPath)

	if len(unclassifiedComments) > 0 {
		if req.NoAgent {
			rep.Summary.Unclassified += len(unclassifiedComments)
		} else if len(avail) == 0 {
			rep.Summary.Unclassified += len(unclassifiedComments)
			rep.Warnings = append(rep.Warnings, "! no agent CLI found — only deterministic rules ran")
		} else {
			provider := ""
			if m.Agents.Default != "" && agents.Binary(m.Agents.Default, env.LookPath) != "" {
				provider = m.Agents.Default
			} else {
				provider = avail[0]
			}
			tier := "fast"
			if t, ok := m.Agents.Tasks["comments"]; ok && t.Tier != "" {
				tier = t.Tier
			}

			packBody, err := loadSkillPackBody(root, m, "antislop-code")
			if err != nil {
				rep.Warnings = append(rep.Warnings, fmt.Sprintf("! could not load antislop-code pack: %v", err))
				rep.Summary.Unclassified += len(unclassifiedComments)
			} else {
				batches := comments.Batches(unclassifiedComments, 150)
				rep.Agent.Provider = provider
				rep.Agent.Tier = tier
				rep.Agent.Batches = len(batches)

				concurrency := req.Concurrency
				if concurrency <= 0 {
					concurrency = 2
				}
				timeout := req.AgentTimeout
				if timeout <= 0 {
					timeout = 3 * time.Minute
				}

				var g errgroup.Group
				g.SetLimit(concurrency)
				var mu sync.Mutex

				for batchIdx, batch := range batches {
					batchIdx := batchIdx
					batch := batch
					g.Go(func() error {
						emit(CleanEvent{Type: "classify", Done: batchIdx + 1, Total: len(batches)})
						prompt := comments.BuildPrompt(packBody, m.Content.Language, batch)
						agentReq := agents.Request{
							Prompt:  prompt,
							Tier:    tier,
							Dir:     root,
							Timeout: timeout,
						}
						out, err := agents.Generate(ctx, env, provider, agentReq)
						var parsed map[string]comments.Classification
						var parseWarnings []string
						if err == nil {
							parsed, parseWarnings, err = comments.ParseClassifications(out, batch)
						}
						// Tolerant retry once on unparseable JSON
						if err != nil {
							outRetry, errRetry := agents.Generate(ctx, env, provider, agentReq)
							if errRetry == nil {
								parsed, parseWarnings, err = comments.ParseClassifications(outRetry, batch)
							}
						}

						mu.Lock()
						defer mu.Unlock()

						if err != nil {
							rep.Warnings = append(rep.Warnings, fmt.Sprintf("batch %d failed: %v — comments kept", batchIdx+1, err))
							for _, c := range batch {
								agentClassifications[c.ID] = comments.Classification{
									ID:         c.ID,
									Action:     "keep",
									Class:      "keep-error",
									Confidence: "high",
									Reason:     err.Error(),
								}
							}
						} else {
							rep.Warnings = append(rep.Warnings, parseWarnings...)
							for id, cls := range parsed {
								agentClassifications[id] = cls
							}
						}
						return nil
					})
				}
				_ = g.Wait()

				// Map classifications to summary and changes
				for i := range filesToProcess {
					fs := &filesToProcess[i]
					for _, c := range unclassifiedComments {
						if c.Path != fs.relPath {
							continue
						}
						cls, ok := agentClassifications[c.ID]
						if !ok || cls.Action == "keep" {
							rep.Summary.Keep++
							continue
						}

						safety := string(comments.SafetyOf(cls))
						if safety == "safe" {
							rep.Summary.Safe++
						} else {
							rep.Summary.Unsafe++
						}

						ch := CleanChange{
							Line:   c.Line,
							Action: cls.Action,
							Class:  cls.Class,
							Safety: safety,
							Source: "agent",
							Text:   cls.Text,
						}
						fs.comments = append(fs.comments, commentWithChange{
							comment: c,
							change:  ch,
						})
					}
				}
			}
		}
	}

	// Prepare file edits and verification
	type touchedFile struct {
		file     CleanFile
		fullPath string
		relPath  string
		src      []byte
		newSrc   []byte
	}
	var touched []touchedFile

	for i, fs := range filesToProcess {
		emit(CleanEvent{Type: "verify", Done: i + 1, Total: len(filesToProcess), Path: fs.relPath})

		var applicableEdits []comments.Edit
		var fileChanges []CleanChange

		for _, item := range fs.comments {
			include := false
			if item.change.Safety == "safe" {
				include = true
			} else if req.Unsafe && item.change.Safety == "unsafe" {
				include = true
			}

			if include {
				fileChanges = append(fileChanges, item.change)
				applicableEdits = append(applicableEdits, planEdit(fs.src, item.comment, item.change.Action, item.change.Text))
			}
		}

		if len(applicableEdits) == 0 {
			rep.Files = append(rep.Files, CleanFile{
				Path:    fs.relPath,
				Status:  "unchanged",
				Changes: fileChanges,
			})
			continue
		}

		newSrc, err := comments.Apply(fs.src, applicableEdits)
		if err != nil {
			rep.Files = append(rep.Files, CleanFile{
				Path:    fs.relPath,
				Status:  "failed",
				Reason:  err.Error(),
				Changes: fileChanges,
			})
			rep.Summary.FailedFiles++
			continue
		}

		if err := comments.VerifyTokens(fs.src, newSrc, fs.lang); err != nil {
			rep.Files = append(rep.Files, CleanFile{
				Path:    fs.relPath,
				Status:  "failed",
				Reason:  "token verification failed",
				Changes: fileChanges,
			})
			rep.Summary.FailedFiles++
			continue
		}

		cFile := CleanFile{
			Path:    fs.relPath,
			Changes: fileChanges,
		}

		if !req.Write {
			cFile.Status = "pending"
			if req.Diff {
				cFile.Diff = textdiff.Lines(fs.src, newSrc)
			}
			rep.Files = append(rep.Files, cFile)
		} else {
			touched = append(touched, touchedFile{
				file:     cFile,
				fullPath: fs.fullPath,
				relPath:  fs.relPath,
				src:      fs.src,
				newSrc:   newSrc,
			})
		}
	}

	// Post-check and Write
	if req.Write {
		hasBiome := false
		if m.Quality != nil {
			if lint, ok := m.Quality["lint"]; ok && strings.ToLower(lint.Provider) == "biome" {
				hasBiome = true
			}
			if format, ok := m.Quality["format"]; ok && strings.ToLower(format.Provider) == "biome" {
				hasBiome = true
			}
		}

		if hasBiome {
			rep.PostCheck = "biome"
		} else {
			rep.PostCheck = "skipped: biome not configured"
		}

		var touchedPaths []string
		for _, tf := range touched {
			touchedPaths = append(touchedPaths, tf.relPath)
		}

		var beforeDiags quality.DiagCounts
		var errBefore error
		if hasBiome && len(touchedPaths) > 0 {
			beforeDiags, errBefore = quality.BiomeDiagnostics(ctx, env, root, touchedPaths)
			if errBefore != nil {
				rep.Warnings = append(rep.Warnings, fmt.Sprintf("! biome pre-check failed: %v", errBefore))
				rep.PostCheck = fmt.Sprintf("skipped: %v", errBefore)
			}
		}

		for i := range touched {
			tf := &touched[i]
			emit(CleanEvent{Type: "write", Done: i + 1, Total: len(touched), Path: tf.relPath})
			if err := atomicWriteFile(tf.fullPath, tf.newSrc); err != nil {
				tf.file.Status = "failed"
				tf.file.Reason = err.Error()
				rep.Summary.FailedFiles++
			} else {
				tf.file.Status = "written"
			}
		}

		if hasBiome && errBefore == nil && len(touchedPaths) > 0 {
			afterDiags, errAfter := quality.BiomeDiagnostics(ctx, env, root, touchedPaths)
			if errAfter == nil {
				regs := quality.Regressions(beforeDiags, afterDiags)
				if len(regs) > 0 {
					regMap := make(map[string]string)
					for _, r := range regs {
						if _, exists := regMap[r.Path]; !exists {
							regMap[r.Path] = r.Category
						}
					}
					for i := range touched {
						tf := &touched[i]
						if cat, regressed := regMap[tf.relPath]; regressed {
							_ = atomicWriteFile(tf.fullPath, tf.src)
							tf.file.Status = "restored"
							rep.Summary.FailedFiles++
							tf.file.Reason = fmt.Sprintf("biome reports %s after the edit", cat)
						}
					}
				}
			}
		}

		for _, tf := range touched {
			rep.Files = append(rep.Files, tf.file)
		}
	}

	slices.SortFunc(rep.Files, func(a, b CleanFile) int {
		return strings.Compare(a.Path, b.Path)
	})

	if initialized {
		head, _ := repo.Head(ctx)
		sessionDir := filepath.Join(root, ".tija", "session")
		if err := os.MkdirAll(sessionDir, 0o755); err == nil {
			sess := struct {
				CleanReport
				Commit string `json:"commit"`
			}{
				CleanReport: rep,
				Commit:      head,
			}
			if data, err := json.MarshalIndent(sess, "", "  "); err == nil {
				_ = atomicWriteFile(filepath.Join(sessionDir, "comments.json"), append(data, '\n'))
			}
		}
	}

	return rep, nil
}

func loadSkillPackBody(root string, m manifest.Manifest, name string) (string, error) {
	p, err := skills.Load(root, m, name)
	if err == nil && p.Body != "" {
		return p.Body, nil
	}
	if data, ok := assets.Skill(name); ok {
		parsed, err := skills.Parse(data)
		if err == nil && parsed.Body != "" {
			return parsed.Body, nil
		}
		return string(data), nil
	}
	return "", fmt.Errorf("skill pack %q not found", name)
}

func atomicWriteFile(target string, data []byte) error {
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".clean-tmp-*")
	if err != nil {
		return os.WriteFile(target, data, 0o644)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if info, err := os.Stat(target); err == nil {
		_ = tmp.Chmod(info.Mode().Perm())
	} else {
		_ = tmp.Chmod(0o644)
	}

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, target); err != nil {
		return os.WriteFile(target, data, 0o644)
	}
	return nil
}
