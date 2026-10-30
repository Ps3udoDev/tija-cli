package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/comments"
	"github.com/Ps3udoDev/tija-cli/internal/sys"
	"github.com/Ps3udoDev/tija-cli/internal/testutil"
)

func commentFakeEnv(handler func(stdin string) string) (sys.Env, *[]agentCall) {
	calls := &[]agentCall{}
	env := fakeEnv()
	look := env.LookPath
	env.LookPath = func(name string) (string, error) {
		if name == "claude" {
			return "/fake/bin/claude", nil
		}
		return look(name)
	}
	env.RunInput = func(ctx context.Context, dir, stdin, name string, args ...string) (string, string, int, error) {
		*calls = append(*calls, agentCall{stdin: stdin, name: name, args: args})
		if handler != nil {
			return handler(stdin), "", 0, nil
		}
		return defaultCommentClassifications(stdin), "", 0, nil
	}
	return env, calls
}

var commentHeaderRx = regexp.MustCompile(`(?m)^### Comment (\S+)`)

func defaultCommentClassifications(stdin string) string {
	matches := commentHeaderRx.FindAllStringSubmatch(stdin, -1)
	type item struct {
		ID         string `json:"id"`
		Action     string `json:"action"`
		Class      string `json:"class"`
		Confidence string `json:"confidence"`
		Reason     string `json:"reason"`
		Text       string `json:"text,omitempty"`
	}
	var items []item
	for _, m := range matches {
		id := m[1]
		switch {
		case strings.Contains(stdin, "Header Section"):
			items = append(items, item{
				ID:         id,
				Action:     "remove",
				Class:      "labeled-separator",
				Confidence: "high",
				Reason:     "layout banner",
			})
		case strings.Contains(stdin, "Maximum 3 attempts"):
			items = append(items, item{
				ID:         id,
				Action:     "keep",
				Class:      "business-rule",
				Confidence: "high",
				Reason:     "explains rate limit rule",
			})
		case strings.Contains(stdin, "calculateOldTax"):
			items = append(items, item{
				ID:         id,
				Action:     "remove",
				Class:      "commented-out-code",
				Confidence: "high",
				Reason:     "dead code comment",
			})
		case strings.Contains(stdin, "This function handles"):
			items = append(items, item{
				ID:         id,
				Action:     "rewrite",
				Class:      "over-explained",
				Confidence: "high",
				Reason:     "shorten narration",
				Text:       "// Handles form submission.",
			})
		default:
			items = append(items, item{
				ID:         id,
				Action:     "keep",
				Class:      "keep-other",
				Confidence: "high",
				Reason:     "default keep",
			})
		}
	}
	data, _ := json.Marshal(items)
	return string(data)
}

func dynamicCommentClassifications(stdin string) string {
	type item struct {
		ID         string `json:"id"`
		Action     string `json:"action"`
		Class      string `json:"class"`
		Confidence string `json:"confidence"`
		Reason     string `json:"reason"`
		Text       string `json:"text,omitempty"`
	}

	lines := strings.Split(stdin, "\n")
	var items []item
	var curID string
	var curText strings.Builder
	inComment := false

	flush := func() {
		if curID == "" {
			return
		}
		txt := curText.String()
		switch {
		case strings.Contains(txt, "Header Section"):
			items = append(items, item{
				ID:         curID,
				Action:     "remove",
				Class:      "labeled-separator",
				Confidence: "high",
				Reason:     "layout banner",
			})
		case strings.Contains(txt, "Maximum 3 attempts"):
			items = append(items, item{
				ID:         curID,
				Action:     "keep",
				Class:      "business-rule",
				Confidence: "high",
				Reason:     "explains rate limit",
			})
		case strings.Contains(txt, "calculateOldTax"):
			items = append(items, item{
				ID:         curID,
				Action:     "remove",
				Class:      "commented-out-code",
				Confidence: "high",
				Reason:     "commented code",
			})
		case strings.Contains(txt, "This function handles"):
			items = append(items, item{
				ID:         curID,
				Action:     "rewrite",
				Class:      "over-explained",
				Confidence: "high",
				Reason:     "trim obvious narration",
				Text:       "// Handles form submission.",
			})
		default:
			items = append(items, item{
				ID:         curID,
				Action:     "keep",
				Class:      "business-rule",
				Confidence: "high",
				Reason:     "keep",
			})
		}
		curID = ""
		curText.Reset()
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "### Comment ") {
			flush()
			curID = strings.TrimSpace(strings.TrimPrefix(line, "### Comment "))
			inComment = false
			continue
		}
		if strings.HasPrefix(line, "- Comment text:") {
			inComment = true
			continue
		}
		if inComment {
			if strings.HasPrefix(line, "- Context after:") || strings.HasPrefix(line, "### Comment ") {
				inComment = false
			} else {
				curText.WriteString(line)
				curText.WriteString("\n")
			}
		}
	}
	flush()

	data, _ := json.Marshal(items)
	return string(data)
}

func setupCommentsRepo(t *testing.T) string {
	t.Helper()
	fixtureDir := filepath.Join("testdata", "comments-repo")
	return testutil.NewRepo(t, fixtureDir)
}

func TestCleanReportOnlyDoesNotWrite(t *testing.T) {
	repo := setupCommentsRepo(t)
	env, _ := commentFakeEnv(dynamicCommentClassifications)

	compPath := filepath.Join(repo, "src", "Component.jsx")
	beforeBytes, err := os.ReadFile(compPath)
	if err != nil {
		t.Fatal(err)
	}

	rep, err := CleanComments(context.Background(), repo, env, CleanRequest{
		Write: false,
		All:   true,
	}, nil)
	if err != nil {
		t.Fatalf("CleanComments: %v", err)
	}

	if rep.Written {
		t.Errorf("Written = true, want false in report-only mode")
	}
	if rep.ExitCode() != 1 {
		t.Errorf("ExitCode = %d, want 1 (safe changes pending)", rep.ExitCode())
	}

	afterBytes, err := os.ReadFile(compPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(beforeBytes) != string(afterBytes) {
		t.Errorf("file modified in report-only mode")
	}

	if rep.Summary.Protected != 1 {
		t.Errorf("Protected = %d, want 1", rep.Summary.Protected)
	}
	if rep.Summary.Safe < 2 {
		t.Errorf("Safe = %d, want >= 2", rep.Summary.Safe)
	}
	if rep.Summary.Keep < 1 {
		t.Errorf("Keep = %d, want >= 1", rep.Summary.Keep)
	}
}

func TestCleanWriteAppliesSafeOnly(t *testing.T) {
	repo := setupCommentsRepo(t)
	env, _ := commentFakeEnv(dynamicCommentClassifications)

	rep, err := CleanComments(context.Background(), repo, env, CleanRequest{
		Write:  true,
		Unsafe: false,
		All:    true,
	}, nil)
	if err != nil {
		t.Fatalf("CleanComments: %v", err)
	}

	if !rep.Written {
		t.Errorf("Written = false, want true")
	}
	if rep.ExitCode() != 0 {
		t.Errorf("ExitCode = %d, want 0", rep.ExitCode())
	}

	compPath := filepath.Join(repo, "src", "Component.jsx")
	content, err := os.ReadFile(compPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)

	// Safe rule separator removed
	if strings.Contains(s, "------------------------------") {
		t.Errorf("separator still present after safe clean")
	}
	// Safe labeled separator removed
	if strings.Contains(s, "Header Section") {
		t.Errorf("Header Section still present after safe clean")
	}
	// Biome-ignore preserved
	if !strings.Contains(s, "biome-ignore") {
		t.Errorf("biome-ignore was removed")
	}
	// Business rule preserved
	if !strings.Contains(s, "Maximum 3 attempts") {
		t.Errorf("business rule comment was removed")
	}
	// Commented-out code (unsafe) preserved
	if !strings.Contains(s, "calculateOldTax") {
		t.Errorf("commented-out code was removed without --unsafe")
	}
	// Rewrite candidate (unsafe) not rewritten
	if strings.Contains(s, "Handles form submission.") {
		t.Errorf("rewrite was applied without --unsafe")
	}
	if !strings.Contains(s, "This function handles") {
		t.Errorf("original over-explained comment missing")
	}
}

func TestCleanWriteUnsafeAppliesAll(t *testing.T) {
	repo := setupCommentsRepo(t)
	env, _ := commentFakeEnv(dynamicCommentClassifications)

	rep, err := CleanComments(context.Background(), repo, env, CleanRequest{
		Write:  true,
		Unsafe: true,
		All:    true,
	}, nil)
	if err != nil {
		t.Fatalf("CleanComments: %v", err)
	}

	if rep.ExitCode() != 0 {
		t.Errorf("ExitCode = %d, want 0", rep.ExitCode())
	}

	compPath := filepath.Join(repo, "src", "Component.jsx")
	content, err := os.ReadFile(compPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)

	// Separator removed
	if strings.Contains(s, "------------------------------") {
		t.Errorf("separator still present")
	}
	// Labeled separator removed
	if strings.Contains(s, "Header Section") {
		t.Errorf("Header Section still present")
	}
	// Commented-out code removed
	if strings.Contains(s, "calculateOldTax") {
		t.Errorf("commented-out code still present with --unsafe")
	}
	// Rewrite applied
	if !strings.Contains(s, "Handles form submission.") {
		t.Errorf("rewrite was not applied with --unsafe")
	}
	// Protected preserved
	if !strings.Contains(s, "biome-ignore") {
		t.Errorf("biome-ignore was removed")
	}
	// Business rule preserved
	if !strings.Contains(s, "Maximum 3 attempts") {
		t.Errorf("business rule comment was removed")
	}
}

func TestCleanNoAgentRulesOnly(t *testing.T) {
	repo := setupCommentsRepo(t)
	env, calls := commentFakeEnv(dynamicCommentClassifications)

	rep, err := CleanComments(context.Background(), repo, env, CleanRequest{
		NoAgent: true,
		All:     true,
	}, nil)
	if err != nil {
		t.Fatalf("CleanComments: %v", err)
	}

	if len(*calls) != 0 {
		t.Errorf("agent called %d times, want 0 with --no-agent", len(*calls))
	}
	if rep.Summary.Unclassified == 0 {
		t.Errorf("Unclassified = 0, want > 0 with --no-agent")
	}
	if rep.Summary.Safe < 1 {
		t.Errorf("Safe = %d, want >= 1 (pure separator rule)", rep.Summary.Safe)
	}
}

func TestCleanNoAgentInstalledFallsBack(t *testing.T) {
	repo := setupCommentsRepo(t)
	env := fakeEnv()
	// LookPath returns error for any agent CLI
	env.LookPath = func(name string) (string, error) {
		return "", os.ErrNotExist
	}

	rep, err := CleanComments(context.Background(), repo, env, CleanRequest{
		All: true,
	}, nil)
	if err != nil {
		t.Fatalf("CleanComments: %v", err)
	}

	if rep.Summary.Unclassified == 0 {
		t.Errorf("Unclassified = 0, want > 0 when no agent installed")
	}

	foundWarning := false
	for _, w := range rep.Warnings {
		if strings.Contains(w, "no agent CLI found") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("warnings %v did not contain 'no agent CLI found'", rep.Warnings)
	}
}

func TestCleanSkipsUntrackedUnlessForce(t *testing.T) {
	repo := setupCommentsRepo(t)
	env, _ := commentFakeEnv(dynamicCommentClassifications)

	untrackedPath := filepath.Join(repo, "src", "Untracked.jsx")
	untrackedContent := "import React from 'react';\n// ------------------------------\nexport const U = () => <div/>;\n"
	if err := os.WriteFile(untrackedPath, []byte(untrackedContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// 1. Without force: file is skipped
	rep1, err := CleanComments(context.Background(), repo, env, CleanRequest{
		Write: true,
		Force: false,
		All:   true,
	}, nil)
	if err != nil {
		t.Fatalf("CleanComments without force: %v", err)
	}

	data1, _ := os.ReadFile(untrackedPath)
	if string(data1) != untrackedContent {
		t.Errorf("untracked file was modified without --force")
	}
	if rep1.Summary.SkippedFiles == 0 {
		t.Errorf("SkippedFiles = 0, want >= 1")
	}

	// 2. With force: file is modified
	rep2, err := CleanComments(context.Background(), repo, env, CleanRequest{
		Write: true,
		Force: true,
		All:   true,
	}, nil)
	if err != nil {
		t.Fatalf("CleanComments with force: %v", err)
	}

	data2, _ := os.ReadFile(untrackedPath)
	if strings.Contains(string(data2), "------------------------------") {
		t.Errorf("untracked file was not cleaned with --force")
	}
	if rep2.ExitCode() != 0 {
		t.Errorf("ExitCode = %d, want 0", rep2.ExitCode())
	}
}

func TestCleanVerificationFailureNotWritten(t *testing.T) {
	repo := setupCommentsRepo(t)
	env, _ := commentFakeEnv(dynamicCommentClassifications)

	compPath := filepath.Join(repo, "src", "Component.jsx")
	beforeBytes, err := os.ReadFile(compPath)
	if err != nil {
		t.Fatal(err)
	}

	origPlan := planEdit
	planEdit = func(src []byte, c comments.Comment, action, text string) comments.Edit {
		edit := origPlan(src, c, action, text)
		// Corrupt the edit by replacing it with extra code tokens that fail token verification
		edit.Replacement = "brokenCodeToken;"
		return edit
	}
	defer func() { planEdit = origPlan }()

	rep, err := CleanComments(context.Background(), repo, env, CleanRequest{
		Write: true,
		All:   true,
	}, nil)
	if err != nil {
		t.Fatalf("CleanComments: %v", err)
	}

	if rep.Summary.FailedFiles == 0 {
		t.Errorf("FailedFiles = 0, want >= 1")
	}
	if rep.ExitCode() != 1 {
		t.Errorf("ExitCode = %d, want 1 on verification failure", rep.ExitCode())
	}

	afterBytes, err := os.ReadFile(compPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(beforeBytes) != string(afterBytes) {
		t.Errorf("file modified despite verification failure")
	}
}

func TestCleanBiomeRegressionRestores(t *testing.T) {
	repo := setupCommentsRepo(t)
	env, _ := commentFakeEnv(dynamicCommentClassifications)

	compPath := filepath.Join(repo, "src", "Component.jsx")
	beforeBytes, err := os.ReadFile(compPath)
	if err != nil {
		t.Fatal(err)
	}

	look := env.LookPath
	env.LookPath = func(name string) (string, error) {
		if name == "biome" {
			return "/fake/bin/biome", nil
		}
		return look(name)
	}

	biomeRuns := 0
	origRun := env.Run
	env.Run = func(ctx context.Context, dir, name string, args ...string) (string, string, int, error) {
		if name == "/fake/bin/biome" || strings.HasSuffix(name, "biome") {
			biomeRuns++
			if biomeRuns == 1 {
				// baseline: no diagnostics
				return "", "", 0, nil
			}
			// second run (after edit): introduces regression
			out := "::error title=lint/correctness/useExhaustiveDependencies,file=src/Component.jsx,line=10,col=1::missing dependency\n"
			return out, "", 1, nil
		}
		return origRun(ctx, dir, name, args...)
	}

	rep, err := CleanComments(context.Background(), repo, env, CleanRequest{
		Write: true,
		All:   true,
	}, nil)
	if err != nil {
		t.Fatalf("CleanComments: %v", err)
	}

	foundRestored := false
	for _, f := range rep.Files {
		if strings.HasSuffix(f.Path, "Component.jsx") && f.Status == "restored" {
			foundRestored = true
			if !strings.Contains(f.Reason, "useExhaustiveDependencies") {
				t.Errorf("reason %q does not mention useExhaustiveDependencies", f.Reason)
			}
		}
	}
	if !foundRestored {
		t.Errorf("expected Component.jsx status restored, files = %+v", rep.Files)
	}

	afterBytes, err := os.ReadFile(compPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(beforeBytes) != string(afterBytes) {
		t.Errorf("file content was not restored to original")
	}
}

func TestCleanDefaultScopeIsDiff(t *testing.T) {
	repo := setupCommentsRepo(t)
	env, _ := commentFakeEnv(dynamicCommentClassifications)

	// Create and checkout branch feat/clean
	testutil.Git(t, repo, "checkout", "-b", "feat/clean")

	// Add a second file and commit it on feat/clean
	changedPath := filepath.Join(repo, "src", "Changed.jsx")
	changedContent := "import React from 'react';\n// ------------------------------\nexport const C = () => <div/>;\n"
	if err := os.WriteFile(changedPath, []byte(changedContent), 0o644); err != nil {
		t.Fatal(err)
	}
	testutil.Git(t, repo, "add", "src/Changed.jsx")
	testutil.Git(t, repo, "commit", "-m", "add Changed.jsx")

	// Default scope (no paths, All: false): diff mode against main
	rep, err := CleanComments(context.Background(), repo, env, CleanRequest{
		Write: false,
	}, nil)
	if err != nil {
		t.Fatalf("CleanComments: %v", err)
	}

	if rep.Scope.Mode != "diff" {
		t.Errorf("Scope.Mode = %q, want diff", rep.Scope.Mode)
	}

	foundChanged := false
	foundOriginal := false
	for _, f := range rep.Files {
		if strings.HasSuffix(f.Path, "Changed.jsx") {
			foundChanged = true
		}
		if strings.HasSuffix(f.Path, "Component.jsx") {
			foundOriginal = true
		}
	}

	if !foundChanged {
		t.Errorf("Changed.jsx was not in scope in diff mode")
	}
	if foundOriginal {
		t.Errorf("unchanged Component.jsx was in scope in diff mode")
	}
}

func TestCleanSavesSession(t *testing.T) {
	repo := setupCommentsRepo(t)
	env, _ := commentFakeEnv(dynamicCommentClassifications)

	rep, err := CleanComments(context.Background(), repo, env, CleanRequest{
		Write: true,
		All:   true,
	}, nil)
	if err != nil {
		t.Fatalf("CleanComments: %v", err)
	}

	sessionFile := filepath.Join(repo, ".tija", "session", "comments.json")
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatalf("comments.json not found: %v", err)
	}

	var sess struct {
		CleanReport
		Commit string `json:"commit"`
	}
	if err := json.Unmarshal(data, &sess); err != nil {
		t.Fatalf("unmarshal session: %v", err)
	}

	head := testutil.Git(t, repo, "rev-parse", "HEAD")
	if sess.Commit != head {
		t.Errorf("session commit = %q, want %q", sess.Commit, head)
	}
	if sess.Summary.Comments != rep.Summary.Comments {
		t.Errorf("session summary comments = %d, want %d", sess.Summary.Comments, rep.Summary.Comments)
	}
}

func TestCleanPathsRelativeToWorkingDir(t *testing.T) {
	repo := setupCommentsRepo(t)
	env, _ := commentFakeEnv(dynamicCommentClassifications)

	rep, err := CleanComments(context.Background(), filepath.Join(repo, "src"), env, CleanRequest{
		Paths:   []string{"Component.jsx"},
		NoAgent: true,
	}, nil)
	if err != nil {
		t.Fatalf("CleanComments: %v", err)
	}
	if rep.Scope.Files != 1 {
		t.Fatalf("Scope.Files = %d, want 1 (path relative to src/)", rep.Scope.Files)
	}
	if len(rep.Files) != 1 || rep.Files[0].Path != "src/Component.jsx" {
		t.Errorf("files = %+v, want src/Component.jsx", rep.Files)
	}
}

func TestCleanRestoredFileFailsExit(t *testing.T) {
	repo := setupCommentsRepo(t)
	env, _ := commentFakeEnv(dynamicCommentClassifications)
	look := env.LookPath
	env.LookPath = func(name string) (string, error) {
		if name == "biome" {
			return "/fake/bin/biome", nil
		}
		return look(name)
	}
	biomeRuns := 0
	origRun := env.Run
	env.Run = func(ctx context.Context, dir, name string, args ...string) (string, string, int, error) {
		if strings.HasSuffix(name, "biome") {
			biomeRuns++
			if biomeRuns == 1 {
				return "", "", 0, nil
			}
			return "::error title=suppressions/unused,file=src/Component.jsx,line=10,col=1::unused\n", "", 1, nil
		}
		return origRun(ctx, dir, name, args...)
	}

	rep, err := CleanComments(context.Background(), repo, env, CleanRequest{Write: true, All: true}, nil)
	if err != nil {
		t.Fatalf("CleanComments: %v", err)
	}
	if rep.Summary.FailedFiles != 1 {
		t.Errorf("FailedFiles = %d, want 1 for the restored file", rep.Summary.FailedFiles)
	}
	if rep.ExitCode() != 1 {
		t.Errorf("ExitCode = %d, want 1 when a file is restored", rep.ExitCode())
	}
}

func TestAtomicWriteFileKeepsMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	p := filepath.Join(t.TempDir(), "bin.js")
	if err := os.WriteFile(p, []byte("#!/usr/bin/env node\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteFile(p, []byte("#!/usr/bin/env node\nx\n")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode = %v, want 0755", info.Mode().Perm())
	}
}
