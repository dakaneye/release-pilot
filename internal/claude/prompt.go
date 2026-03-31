package claude

import (
	"fmt"
	"strings"

	"github.com/dakaneye/release-pilot/internal/git"
	gh "github.com/dakaneye/release-pilot/internal/github"
)

type PromptInput struct {
	RepoOwner  string
	RepoName   string
	CurrentTag string
	PRs        []gh.PR
	Commits    []git.Commit
	Diffs      string
}

func SystemPrompt() string {
	return `You are a release engineer analyzing changes for a software release.

Your job:
1. Determine the appropriate semver bump level (major, minor, or patch) based on the changes.
2. Write human-readable release notes in markdown.

Rules for semver bump:
- major: breaking changes to public API, removed features, incompatible changes
- minor: new features, new capabilities, non-breaking additions
- patch: bug fixes, documentation, dependency updates, internal refactoring

Rules for release notes:
- Group changes under these headings (omit empty groups): Breaking Changes, Features, Fixes, Other
- Write for end users: explain what changed and why it matters, not implementation details
- Reference PR numbers as links: [#N](https://github.com/{owner}/{repo}/pull/N)
- Be concise: one line per change unless it needs more context
- If there are breaking changes, put them first with clear migration guidance

Respond with ONLY a JSON object in this exact format:
{
  "bump": "major" | "minor" | "patch",
  "notes": "markdown release notes"
}`
}

func BuildUserPrompt(input PromptInput) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Repository: %s/%s\n", input.RepoOwner, input.RepoName)
	fmt.Fprintf(&b, "Current version: %s\n\n", input.CurrentTag)

	if len(input.PRs) > 0 {
		b.WriteString("## Merged Pull Requests\n\n")
		for _, pr := range input.PRs {
			fmt.Fprintf(&b, "### #%d: %s\n", pr.Number, pr.Title)
			if pr.Body != "" {
				fmt.Fprintf(&b, "%s\n", pr.Body)
			}
			b.WriteString("\n")
		}
	} else {
		b.WriteString("No pull requests found since last release.\n\n")
	}

	if len(input.Commits) > 0 {
		b.WriteString("## Commits\n\n")
		for _, c := range input.Commits {
			fmt.Fprintf(&b, "- %s (%s)\n", c.Subject, c.Hash[:7])
		}
		b.WriteString("\n")
	}

	if input.Diffs != "" {
		b.WriteString("## Diffs\n\n")
		b.WriteString(input.Diffs)
		b.WriteString("\n")
	}

	return b.String()
}
