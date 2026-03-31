# release-pilot

[![CI](https://github.com/dakaneye/release-pilot/actions/workflows/ci.yml/badge.svg)](https://github.com/dakaneye/release-pilot/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/dakaneye/release-pilot)](https://goreportcard.com/report/github.com/dakaneye/release-pilot)
[![Go Reference](https://pkg.go.dev/badge/github.com/dakaneye/release-pilot.svg)](https://pkg.go.dev/github.com/dakaneye/release-pilot)

Orchestrate releases with AI-powered release notes. A single Go CLI that detects your ecosystem, determines the version bump, writes human-readable release notes via Claude, creates a GitHub release, and optionally signs artifacts with cosign.

## Features

- **Multi-ecosystem** — Go (with goreleaser), Python, and Node.js
- **AI-powered version bumping** — Claude analyzes PRs and commits to determine major/minor/patch
- **Human-readable release notes** — grouped by category, written for end users
- **GitHub Action** — simple (batteries-included) and advanced (install-only) modes
- **Idempotent pipeline** — safe to re-run if a step fails mid-release
- **Keyless signing** — optional cosign signing with Sigstore

## Install

```bash
go install github.com/dakaneye/release-pilot/cmd/release-pilot@latest
```

Or download a binary from [Releases](https://github.com/dakaneye/release-pilot/releases).

## Quick Start

```bash
# Generate config
release-pilot init

# Preview a release (no changes made)
export ANTHROPIC_API_KEY=your-key
export GITHUB_TOKEN=your-token
release-pilot ship --dry-run

# Ship it
release-pilot ship
```

## GitHub Action

### Simple mode

```yaml
- uses: dakaneye/release-pilot@v1
  with:
    anthropic-api-key: ${{ secrets.ANTHROPIC_API_KEY }}
```

### Advanced mode

```yaml
- uses: dakaneye/release-pilot/setup@v1
- run: release-pilot ship --sign
  env:
    ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
```

## Configuration

`.release-pilot.yaml` in your repo root:

```yaml
ecosystem: auto           # auto | go | python | node
model: claude-sonnet-4-6  # any Claude model

notes:
  include-diffs: false     # send full diffs to Claude (more tokens)

github:
  draft: false
  prerelease: false
```

## CLI Reference

```
release-pilot ship [flags]

Flags:
  --dry-run          Preview without making changes
  --sign             Sign artifacts with cosign (keyless)
  --step <name>      Run a single step (detect, bump, notes, release, sign)
  --version <ver>    Override version instead of AI-determined bump
  --force            Reset pipeline state and start fresh
  --config <path>    Path to config file
```

## How It Works

1. **Detect** — identifies ecosystem from manifest files (go.mod, package.json, pyproject.toml)
2. **Bump** — gathers PRs and commits since last tag, Claude determines semver level
3. **Notes** — Claude generates grouped, human-readable release notes (same API call as bump)
4. **Release** — creates git tag and GitHub release (delegates to goreleaser for Go if configured)
5. **Sign** — keyless cosign signing of release artifacts (if `--sign` is passed)

## License

[MIT](LICENSE)
