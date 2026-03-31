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
	return `You are a release engineer. Analyze the changes provided and return a JSON object with two fields.

RESPONSE FORMAT — Return ONLY a raw JSON object. No markdown fences, no commentary, no text before or after:
{"bump":"<level>","notes":"<markdown>"}

FIELD: bump
Determine the single highest semver bump level across all changes:
- "major": Removed or renamed public API (functions, types, endpoints, CLI flags), changed function signatures in breaking ways, changed default behavior that existing users depend on
- "minor": New features, new API surface, new commands/flags/endpoints, deprecations (without removal), new optional parameters
- "patch": Bug fixes, performance improvements, documentation, dependency updates, CI/test/refactoring changes, anything that does not change public-facing behavior
When changes span multiple levels, use the highest. If uncertain between two levels, choose the lower one.

FIELD: notes
Write concise release notes in markdown for end users:
- Use only these H2 headings (omit empty ones): Breaking Changes, Features, Fixes, Dependencies, Other
- Breaking Changes goes first when present. Include migration guidance for each breaking change.
- One bullet per change. Describe WHAT changed and WHY it matters, not how it was implemented.
- Reference PRs as links: [#N](https://github.com/OWNER/REPO/pull/N) using the actual owner/repo from the input.
- When no PRs exist, reference commits by short hash.
- Group dependency update PRs (Dependabot, Renovate, deps in title) under Dependencies.
- Omit CI-only, test-only, and internal tooling changes unless they affect end users.
- Do not invent changes absent from the input.`
}

func BuildUserPrompt(input PromptInput) string {
	var b strings.Builder

	b.WriteString("Analyze the following changes and produce the JSON response.\n\n")
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
		b.WriteString("No pull requests found. Use commits below as the sole source of changes.\n\n")
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
