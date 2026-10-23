package engine

import (
	"fmt"
	"strings"

	"github.com/Ps3udoDev/tija-cli/internal/prrules"
	"github.com/Ps3udoDev/tija-cli/internal/quality"
)

var languageNames = map[string]string{
	"es": "Spanish", "en": "English", "pt": "Portuguese", "fr": "French", "de": "German", "it": "Italian",
}

func languageName(code string) string {
	if code == "" {
		code = "es"
	}
	if n, ok := languageNames[strings.ToLower(code)]; ok {
		return n
	}
	return code
}

// BuildPRPrompt renders the agent prompt. Everything the agent needs is in
// it: the agent only writes, it does not run tools. problems, when not
// empty, are the checks the previous answer failed.
func BuildPRPrompt(c PRContext, problems []prrules.Problem) string {
	pr := c.Manifest.PullRequest
	lang := languageName(c.Manifest.Content.Language)
	var b strings.Builder
	section := func(title string) { fmt.Fprintf(&b, "\n# %s\n\n", title) }

	fmt.Fprintf(&b, "You are writing the title and description of a pull request for this repository. "+
		"Write all the content in %s. Use only the facts given below; you do not need to run any tool.\n", lang)

	section("Repository rules")
	b.WriteString(strings.TrimSpace(c.Rules) + "\n")

	section("Pull request template")
	b.WriteString("Fill every section of this template. Keep the headings exactly as they are and remove the HTML comments.\n\n")
	b.WriteString(strings.TrimSpace(c.Template) + "\n")
	if len(pr.RequiredSections) > 0 {
		fmt.Fprintf(&b, "\nRequired sections (each needs at least %d characters of useful content): %s.\n",
			pr.MinSectionLength, strings.Join(pr.RequiredSections, ", "))
	}

	section("Title convention")
	b.WriteString("Conventional Commits: `type(scope): description`.\n")
	fmt.Fprintf(&b, "- Allowed types: %s.\n", strings.Join(prrules.Types, ", "))
	if pr.Title.RequireScope == nil || *pr.Title.RequireScope {
		b.WriteString("- The scope is required.\n")
	}
	if pr.Title.MaxLength > 0 {
		fmt.Fprintf(&b, "- At most %d characters.\n", pr.Title.MaxLength)
	}
	b.WriteString("- No final period.\n")
	fmt.Fprintf(&b, "- The description is written in %s and centred on the main change.\n", lang)

	section("Branch")
	fmt.Fprintf(&b, "- Branch: `%s`\n- Target: `%s`\n", c.Branch, c.Target)
	if c.Verdict.Suggested != "" {
		fmt.Fprintf(&b, "- The branch name does not follow the convention; its suggested name is `%s`. Infer type and scope from it.\n", c.Verdict.Suggested)
	}
	if c.IssueNumber > 0 {
		fmt.Fprintf(&b, "- Issue: #%d. Put `Closes #%d` in the `## Referencias` section.\n", c.IssueNumber, c.IssueNumber)
	} else {
		b.WriteString("- No issue number is known. Do not invent one.\n")
	}

	section(fmt.Sprintf("Commits (%d, oldest first)", len(c.Commits)))
	for _, cm := range c.Commits {
		fmt.Fprintf(&b, "- %s\n", cm.Subject)
		if cm.Body != "" {
			for _, l := range strings.Split(cm.Body, "\n") {
				fmt.Fprintf(&b, "    %s\n", l)
			}
		}
	}

	section("Changed files")
	for _, f := range c.Files {
		if f.Binary {
			fmt.Fprintf(&b, "- %s (binary)\n", f.Path)
		} else {
			fmt.Fprintf(&b, "- %s (+%d -%d)\n", f.Path, f.Added, f.Deleted)
		}
	}

	section("Diff")
	if c.Diff == "" {
		b.WriteString("(no textual diff)\n")
	} else {
		b.WriteString("```diff\n" + strings.TrimRight(c.Diff, "\n") + "\n```\n")
	}
	if len(c.Truncated) > 0 {
		b.WriteString("\nThese files are left out of the diff (lockfiles, generated, binary or too large); only their line counts above are known:\n")
		for _, p := range c.Truncated {
			fmt.Fprintf(&b, "- %s\n", p)
		}
	}

	section("Verified checks")
	if len(c.Tests) == 0 {
		b.WriteString("No automated checks were verified for this commit. Do not claim any test was run. " +
			"Say so in the tests section.\n")
	} else {
		b.WriteString("These are the only checks verified on this exact commit. Cite them in the tests section and nothing else:\n")
		for _, r := range c.Tests {
			b.WriteString("- " + describeResult(r) + "\n")
		}
	}

	section("Prohibitions")
	b.WriteString("- Do not invent tests, endpoints or features that are not in the commits or the diff.\n" +
		"- Keep technical names (DTOs, endpoints, classes, hooks, services, files) exactly as they appear in the code.\n" +
		"- Summarise the commits by feature; do not copy the commit messages literally.\n" +
		"- Write `N/A` in the `## Evidencias` section.\n" +
		"- Do not use a generic title.\n")

	section("Output format")
	b.WriteString("Answer with exactly these two blocks and nothing else:\n\n" +
		"<<<TITLE\ntype(scope): description\nTITLE>>>\n<<<BODY\n## ...\nBODY>>>\n")

	if len(problems) > 0 {
		section("Corrections")
		b.WriteString("Your previous answer failed these checks:\n")
		for _, p := range problems {
			fmt.Fprintf(&b, "- %s: %s\n", p.Field, p.Message)
		}
		b.WriteString("Fix all of them in the new answer.\n")
	}
	return b.String()
}

func describeResult(r quality.StepResult) string {
	name := r.Command
	if name == "" {
		name = r.Provider
	}
	s := fmt.Sprintf("`%s`: %s", name, r.Status)
	switch {
	case r.TimedOut:
		s += " (timed out)"
	case r.Status == quality.StatusFailed:
		s += fmt.Sprintf(" (exit code %d)", r.ExitCode)
	}
	if t := r.Tests; t != nil {
		s += fmt.Sprintf(", %d tests: %d passed, %d failed, %d skipped", t.Total, t.Passed, t.Failed, t.Skipped)
	}
	return s
}
