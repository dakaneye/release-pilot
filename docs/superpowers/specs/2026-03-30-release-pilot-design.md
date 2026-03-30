# release-pilot v0.1.0 Design Spec

## Problem

Release workflows are fragmented across ecosystems. Go uses goreleaser, Node uses semantic-release, Python uses poetry/flit. No single tool orchestrates version bumping, release notes, GitHub releases, and signing across all three. Auto-generated changelogs from commit messages are noisy — real release notes should explain what changed and why, for humans.

## Solution

Go CLI that orchestrates the full release flow. Single `ship` command detects the ecosystem, determines the version bump via Claude, generates human-readable release notes, creates a GitHub release, and optionally signs artifacts with cosign. Ships as a single binary and a GitHub Action.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Ecosystems | Go, Python, Node from v0.1.0 | Target breadth from the start |
| Execution context | CI-first, non-interactive | Best practice: releases happen in CI |
| Distribution | CLI binary + GitHub Action | CLI is the core, Action is a thin wrapper |
| Action modes | Simple (batteries-included) + Advanced (just the CLI) | Lower barrier for newcomers, stay out of the way for power users |
| Version bump strategy | AI-determined from PRs/commits | Claude analyzes changes, determines major/minor/patch |
| Release notes input | PR titles + bodies + commits (default), full diffs (opt-in) | PRs + commits balance signal vs. noise; diffs are token-heavy |
| Go integration | Delegate to goreleaser when config exists | Don't replace goreleaser, complement it |
| Python/Node scope | Version bump + GitHub release only | Publishing to PyPI/npm is the user's responsibility |
| Signing | Keyless cosign only, via `--sign` flag | Single option, no config needed |
| Claude auth | `ANTHROPIC_API_KEY` env var | Standard for CI secrets |
| Model selection | Configurable, default Sonnet | Cost/quality default, let users choose Opus/Haiku |
| Architecture | Orchestrated steps with single entry point | One command UX, pipeline internals, idempotent re-runs |

## CLI Interface

### Commands

```
release-pilot ship [--step <name>] [--dry-run] [--sign] [--version <override>] [--force]
release-pilot init       # generates .release-pilot.yaml
release-pilot version    # prints CLI version
```

### Config File

`.release-pilot.yaml` in repo root:

```yaml
ecosystem: auto       # only needed for ambiguous repos (monorepo)
model: claude-sonnet-4-6

notes:
  include-diffs: false

github:
  draft: false
  prerelease: false
```

### Environment Variables

- `ANTHROPIC_API_KEY` — required
- `GITHUB_TOKEN` — required (auto-provided in GitHub Actions)
- `RELEASE_PILOT_MODEL` — optional, overrides config for one-off runs
- `RELEASE_PILOT_CONFIG` — optional, path override for config file

### Precedence

Flags > env vars > config file > defaults.

## Pipeline Steps

The `ship` command runs these steps in order. Each step is idempotent.

### 1. detect

Identify ecosystem from manifest files:
- `go.mod` → Go
- `package.json` → Node
- `pyproject.toml` → Python

If goreleaser config (`.goreleaser.yaml` / `.goreleaser.yml`) exists alongside `go.mod`, note it for the release step. Fail if multiple manifests found and `ecosystem` is not set in config.

### 2. bump

- Find latest git tag
- Gather merged PRs since that tag via GitHub API (PRs merged after the tag's timestamp)
- Gather commit messages since that tag via `git log`
- Call Claude to determine semver bump level (major/minor/patch)
- For Python/Node: update the version in the manifest file
- For Go: no manifest update (version lives in git tags only)

### 3. notes

Generated in the same Claude API call as the bump. Claude produces markdown release notes from the PR/commit context.

### 4. release

- For Python/Node: commit the version change, create git tag, push both
- For Go: create git tag, push it (no version commit)
- **Go with goreleaser:** run `goreleaser release`, then PATCH the GitHub release body with the AI-generated notes
- **Go without goreleaser / Python / Node:** create GitHub release via GitHub API with the notes and tag

### 5. sign

If `--sign` is passed, run `cosign sign-blob` (keyless/Fulcio) on release artifacts.

### State Tracking

A `.release-pilot-state.json` file in `$RUNNER_TEMP` or `/tmp` tracks completed steps per run. Re-running `ship` skips completed steps. `--force` resets state.

### Combined API Call

The bump and notes steps share a single Claude API call. The prompt asks Claude to both determine the version bump and write release notes. This saves a round-trip and keeps reasoning coherent.

## Ecosystem-Specific Behavior

### Go

- **Version source:** git tags (Go modules don't store version in `go.mod`)
- **Bump:** create new git tag
- **With goreleaser config:** run goreleaser (which creates the release), then patch release body with AI notes
- **Without goreleaser:** create tag, create GitHub release with notes

Sequencing for goreleaser: tag must exist before goreleaser runs. AI notes are generated before goreleaser but applied after it creates the release.

### Python

- **Version source:** `pyproject.toml` — `[project].version` or `[tool.poetry].version`
- **Bump:** update the version field, commit, tag

### Node

- **Version source:** `package.json` — `version` field
- **Bump:** update `package.json` and `package-lock.json` (if present), commit, tag

### Common

After bumping, the tool creates a git tag and pushes it. For Python/Node, it also commits the manifest version change before tagging. Then proceeds to release and sign.

## Claude API Integration

### Prompt Design

Single API call with structured JSON output containing `bump` (major/minor/patch) and `notes` (markdown string).

The prompt instructs Claude to:
- Analyze PRs and commits to determine the appropriate semver bump
- Group changes by category (breaking changes, features, fixes, other)
- Write for end users — what changed and why it matters
- Call out breaking changes prominently at the top
- Link PR numbers back to GitHub

**The prompt is a critical quality gate.** It will be engineered using `/skill-creator` and the `prompt-engineer` agent, with dedicated evals measuring bump accuracy and notes quality.

### Input Context

Default: PR titles, PR bodies, commit messages.
Opt-in (`notes.include-diffs: true`): full diffs per PR.

### Token Management

PRs + commits are naturally bounded (only since last tag). If opt-in diffs push input beyond context limits, truncate diffs starting from oldest PR and log a warning.

### Error Handling

- API call failure: step fails, retryable on next `ship` invocation
- Unparseable response: retry once, then fail with raw response logged

## GitHub Action

### Simple Mode (batteries-included)

```yaml
- uses: dakaneye/release-pilot@v1
  with:
    anthropic-api-key: ${{ secrets.ANTHROPIC_API_KEY }}
```

Handles checkout, installs binary, runs `release-pilot ship`, posts job summary.

### Advanced Mode (just the CLI)

```yaml
- uses: dakaneye/release-pilot/setup@v1
- run: release-pilot ship --sign
```

Only installs the binary. User composes their own workflow.

### Inputs

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `anthropic-api-key` | yes | — | Anthropic API key |
| `model` | no | `claude-sonnet-4-6` | Claude model |
| `sign` | no | `false` | Enable cosign signing |
| `draft` | no | `false` | Create draft release |
| `prerelease` | no | `false` | Mark as prerelease |
| `include-diffs` | no | `false` | Include full diffs in prompt |
| `args` | no | — | Pass-through CLI flags |

### Outputs

| Output | Description |
|--------|-------------|
| `version` | Released version (e.g., `1.2.0`) |
| `tag` | Git tag created (e.g., `v1.2.0`) |
| `release-url` | GitHub release URL |
| `release-notes` | Generated markdown |

## Testing Strategy

### Unit Tests

- Ecosystem detection (given files → detected ecosystem)
- Version parsing and bumping per ecosystem (tag parsing, manifest updates)
- State tracking (idempotency, skip completed steps, `--force` reset)
- Claude response parsing (valid JSON, malformed response, retry)

### CLI Acceptance Tests

Build the binary, run it against a real temp Git repo with real commit history, real tags, real PR-shaped data. Mock only the Claude API (HTTP stub with realistic responses). Assert:
- Correct tag created
- Correct manifest files modified with correct version
- Correct GitHub API calls made with correct payload
- Release notes contain expected content

### Error Path Tests

Not just "does it return an error" but "does it tell the user what went wrong and what to do." Cover:
- Missing `ANTHROPIC_API_KEY`
- No git tags in repo
- Ambiguous ecosystem (multiple manifests, no config)
- Claude returns garbage JSON
- Network failure mid-release
- GitHub API errors (rate limit, auth failure)

### Idempotency Tests

Run `ship`, kill after step 3, run `ship` again. Verify it picks up where it left off and the end result is identical to an uninterrupted run.

### Golden File Tests

Given a known set of PRs/commits, assert the exact prompt sent to Claude matches the expected output. Catches prompt construction drift.

### Prompt Evals

Separate eval suite using real-world PR/commit datasets. Measure:
- Bump accuracy: did Claude pick the right semver level?
- Notes quality: readable, correct grouping, no hallucinated changes

Run on schedule or manually (costs API tokens).

### GitHub Action Tests

- Action syntax validation
- Simple mode and advanced mode against a test repo

### The Bar

If every test passes, a user can run `release-pilot ship` in a real repo and get a real release. No gaps between "tests green" and "actually works."

## Tech Stack

- **Language:** Go
- **CLI framework:** cobra
- **Claude integration:** Anthropic Go SDK
- **GitHub integration:** GitHub API (via go-github or direct HTTP)
- **Signing:** cosign CLI
- **Build/release of release-pilot itself:** goreleaser + Homebrew

## Out of Scope for v0.1.0

- Publishing to package registries (npm, PyPI)
- Key-based signing
- Custom prompt templates
- Monorepo support (multiple packages in one repo)
- Non-GitHub forges (GitLab, Bitbucket)
