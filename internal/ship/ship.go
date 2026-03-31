package ship

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dakaneye/release-pilot/internal/claude"
	"github.com/dakaneye/release-pilot/internal/config"
	"github.com/dakaneye/release-pilot/internal/detect"
	"github.com/dakaneye/release-pilot/internal/git"
	"github.com/dakaneye/release-pilot/internal/github"
	"github.com/dakaneye/release-pilot/internal/pipeline"
	"github.com/dakaneye/release-pilot/internal/sign"
	"github.com/dakaneye/release-pilot/internal/version"
)

// Options controls the behavior of the ship command.
type Options struct {
	Step        string
	DryRun      bool
	Sign        bool
	VersionOver string
	Force       bool
	ConfigPath  string
}

// Run orchestrates the full release pipeline.
func Run(dir string, opts Options) error {
	cfg := config.Load(opts.ConfigPath)

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY is required: set it in your environment or CI secrets")
	}

	ghToken := os.Getenv("GITHUB_TOKEN")
	if ghToken == "" {
		return fmt.Errorf("GITHUB_TOKEN is required: set it in your environment or CI secrets")
	}

	claudeBaseURL := os.Getenv("ANTHROPIC_BASE_URL")
	ghBaseURL := os.Getenv("GITHUB_API_URL")

	claudeClient := claude.NewClient(apiKey, cfg.Model, claudeBaseURL)
	ghClient := github.NewClient(ghToken, ghBaseURL)

	tmpDir := os.Getenv("RUNNER_TEMP")
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}
	statePath := filepath.Join(tmpDir, "release-pilot-state.json")

	remoteURL, err := git.RemoteURL(dir)
	if err != nil {
		return fmt.Errorf("get git remote: %w", err)
	}

	owner, repo, err := parseRemote(remoteURL)
	if err != nil {
		return fmt.Errorf("parse remote URL %q: %w", remoteURL, err)
	}

	steps := []pipeline.Step{
		{
			Name: "detect",
			Run: func(ctx *pipeline.Context) error {
				result, err := detect.Ecosystem(dir, cfg.Ecosystem)
				if err != nil {
					return err
				}
				log.Printf("detected ecosystem: %s", result.Name)
				ctx.State.Set("ecosystem", result.Name)
				ctx.State.Set("has-goreleaser", fmt.Sprintf("%t", result.HasGoreleaser))
				if result.ManifestPath != "" {
					ctx.State.Set("manifest-path", result.ManifestPath)
				}
				return nil
			},
		},
		{
			Name: "bump",
			Run: func(ctx *pipeline.Context) error {
				latestTag, err := git.LatestTag(dir)
				if err != nil {
					return fmt.Errorf("get latest tag: %w", err)
				}
				log.Printf("latest tag: %s", latestTag)

				current, err := version.ParseTag(latestTag)
				if err != nil {
					return fmt.Errorf("parse tag %s: %w", latestTag, err)
				}

				tagTime, err := git.TagTimestamp(dir, latestTag)
				if err != nil {
					return fmt.Errorf("get tag timestamp: %w", err)
				}

				prs, err := ghClient.MergedPRsSince(owner, repo, tagTime)
				if err != nil {
					return fmt.Errorf("fetch merged PRs: %w", err)
				}

				commits, err := git.CommitsSince(dir, latestTag)
				if err != nil {
					return fmt.Errorf("get commits since %s: %w", latestTag, err)
				}

				if len(prs) == 0 && len(commits) == 0 {
					return fmt.Errorf("nothing to release: no PRs or commits since %s", latestTag)
				}

				input := claude.PromptInput{
					RepoOwner:  owner,
					RepoName:   repo,
					CurrentTag: latestTag,
					PRs:        prs,
					Commits:    commits,
				}

				if cfg.Notes.IncludeDiffs {
					diffs, err := git.DiffSince(dir, latestTag)
					if err != nil {
						return fmt.Errorf("get diffs: %w", err)
					}
					input.Diffs = diffs
				}

				var bumpLevel string
				var notes string

				if opts.VersionOver != "" {
					// Use override version directly; still call AI for notes.
					analysis, err := claudeClient.Analyze(input)
					if err != nil {
						return fmt.Errorf("claude analysis: %w", err)
					}
					notes = analysis.Notes

					overVer, err := version.ParseTag(opts.VersionOver)
					if err != nil {
						return fmt.Errorf("parse version override %s: %w", opts.VersionOver, err)
					}
					ctx.State.Set("tag", overVer.Tag())
					ctx.State.Set("bump", "override")
					ctx.State.Set("notes", notes)
				} else {
					analysis, err := claudeClient.Analyze(input)
					if err != nil {
						return fmt.Errorf("claude analysis: %w", err)
					}
					bumpLevel = analysis.Bump
					notes = analysis.Notes

					next := current.Bump(bumpLevel)
					ctx.State.Set("tag", next.Tag())
					ctx.State.Set("bump", bumpLevel)
					ctx.State.Set("notes", notes)
				}

				log.Printf("next version: %s (bump: %s)", ctx.State.Get("tag"), ctx.State.Get("bump"))

				// Update manifest for Python/Node ecosystems.
				eco := ctx.State.Get("ecosystem")
				manifest := ctx.State.Get("manifest-path")
				if manifest != "" && (eco == "python" || eco == "node") {
					tag := ctx.State.Get("tag")
					ver, _ := version.ParseTag(tag)
					if err := version.UpdateManifest(manifest, eco, ver.String()); err != nil {
						return fmt.Errorf("update manifest: %w", err)
					}
					log.Printf("updated %s manifest: %s", eco, manifest)
				}

				return nil
			},
		},
		{
			Name: "notes",
			Run: func(ctx *pipeline.Context) error {
				notes := ctx.State.Get("notes")
				log.Printf("release notes generated (%d chars)", len(notes))
				return nil
			},
		},
		{
			Name: "release",
			Run: func(ctx *pipeline.Context) error {
				tag := ctx.State.Get("tag")
				notes := ctx.State.Get("notes")
				eco := ctx.State.Get("ecosystem")
				hasGoreleaser := ctx.State.Get("has-goreleaser") == "true"

				if opts.DryRun {
					log.Printf("[dry-run] would create release %s", tag)
					log.Printf("[dry-run] release notes:\n%s", notes)
					return nil
				}

				// For Python/Node, commit and push the manifest change.
				if eco == "python" || eco == "node" {
					if err := git.CommitAll(dir, fmt.Sprintf("release: %s", tag)); err != nil {
						return fmt.Errorf("commit manifest: %w", err)
					}
					if err := git.Push(dir); err != nil {
						return fmt.Errorf("push manifest commit: %w", err)
					}
				}

				if err := git.CreateTag(dir, tag); err != nil {
					return fmt.Errorf("create tag: %w", err)
				}
				if err := git.PushTag(dir, tag); err != nil {
					return fmt.Errorf("push tag: %w", err)
				}

				if hasGoreleaser {
					if err := runGoreleaser(); err != nil {
						return fmt.Errorf("goreleaser: %w", err)
					}
					if err := ghClient.EditReleaseBody(owner, repo, tag, notes); err != nil {
						return fmt.Errorf("edit release body: %w", err)
					}
				} else {
					releaseURL, err := ghClient.CreateRelease(owner, repo, github.ReleaseParams{
						Tag:        tag,
						Name:       tag,
						Body:       notes,
						Draft:      cfg.GitHub.Draft,
						Prerelease: cfg.GitHub.Prerelease,
					})
					if err != nil {
						return fmt.Errorf("create release: %w", err)
					}
					ctx.State.Set("release-url", releaseURL)
					log.Printf("release created: %s", releaseURL)
				}

				return nil
			},
		},
		{
			Name: "sign",
			Run: func(ctx *pipeline.Context) error {
				tag := ctx.State.Get("tag")
				if opts.DryRun {
					log.Printf("[dry-run] would sign %s", tag)
					return nil
				}
				return sign.Run(opts.Sign, tag, owner, repo)
			},
		},
	}

	p := pipeline.New(statePath, steps)

	if opts.Step != "" {
		return p.RunStep(opts.Step, opts.Force)
	}
	return p.Run(opts.Force)
}

// parseRemote extracts owner and repo from a git remote URL.
// Supports SSH (git@github.com:owner/repo.git) and HTTPS (https://github.com/owner/repo.git).
func parseRemote(url string) (string, string, error) {
	// Strip trailing .git suffix.
	url = strings.TrimSuffix(url, ".git")

	// SSH format: git@github.com:owner/repo
	if strings.Contains(url, ":") && strings.HasPrefix(url, "git@") {
		parts := strings.SplitN(url, ":", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("unexpected SSH remote format")
		}
		segments := strings.Split(parts[1], "/")
		if len(segments) != 2 {
			return "", "", fmt.Errorf("expected owner/repo in SSH remote, got %q", parts[1])
		}
		return segments[0], segments[1], nil
	}

	// HTTPS format: https://github.com/owner/repo
	parts := strings.Split(url, "/")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("unexpected remote URL format: %s", url)
	}
	repo := parts[len(parts)-1]
	owner := parts[len(parts)-2]
	return owner, repo, nil
}

// runGoreleaser executes goreleaser release --clean.
func runGoreleaser() error {
	cmd := exec.Command("goreleaser", "release", "--clean")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
