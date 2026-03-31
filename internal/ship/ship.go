package ship

import (
	"context"
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
	Tag         string // pre-existing tag (CI mode: tag already pushed, use previous tag as baseline)
	Force       bool
	ConfigPath  string
}

// Run orchestrates the full release pipeline.
func Run(ctx context.Context, dir string, opts Options) error {
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

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

	remoteURL, err := git.RemoteURL(ctx, dir)
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
			Run: func(sctx *pipeline.StepContext) error {
				result, err := detect.Ecosystem(dir, cfg.Ecosystem)
				if err != nil {
					return err
				}
				log.Printf("detected ecosystem: %s", result.Name)
				sctx.State.Set("ecosystem", result.Name)
				sctx.State.Set("has-goreleaser", fmt.Sprintf("%t", result.HasGoreleaser))
				if result.ManifestPath != "" {
					sctx.State.Set("manifest-path", result.ManifestPath)
				}
				return nil
			},
		},
		{
			Name: "bump",
			Run: func(sctx *pipeline.StepContext) error {
				var latestTag string
				if opts.Tag != "" {
					// CI mode: tag already exists, find the previous one as baseline
					prev, err := git.PreviousTag(ctx, dir, opts.Tag, "")
					if err != nil {
						return fmt.Errorf("find previous tag before %s: %w", opts.Tag, err)
					}
					latestTag = prev
					sctx.State.Set("tag", opts.Tag)
				} else {
					tag, err := git.LatestTag(ctx, dir, "")
					if err != nil {
						return fmt.Errorf("get latest tag: %w", err)
					}
					latestTag = tag
				}
				log.Printf("latest tag: %s", latestTag)

				current, err := version.ParseTag(latestTag)
				if err != nil {
					return fmt.Errorf("parse tag %s: %w", latestTag, err)
				}

				tagTime, err := git.TagTimestamp(ctx, dir, latestTag)
				if err != nil {
					return fmt.Errorf("get tag timestamp: %w", err)
				}

				prs, err := ghClient.MergedPRsSince(ctx, owner, repo, tagTime)
				if err != nil {
					return fmt.Errorf("fetch merged PRs: %w", err)
				}

				commits, err := git.CommitsSince(ctx, dir, latestTag)
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
					diffs, err := git.DiffSince(ctx, dir, latestTag)
					if err != nil {
						return fmt.Errorf("get diffs: %w", err)
					}
					input.Diffs = diffs
				}

				analysis, err := claudeClient.Analyze(ctx, input)
				if err != nil {
					return fmt.Errorf("claude analysis: %w", err)
				}

				sctx.State.Set("notes", analysis.Notes)

				if opts.Tag != "" {
					// CI mode: tag already set above from --tag flag
					sctx.State.Set("bump", analysis.Bump)
				} else if opts.VersionOver != "" {
					overVer, err := version.ParseTag(opts.VersionOver)
					if err != nil {
						return fmt.Errorf("parse version override %s: %w", opts.VersionOver, err)
					}
					sctx.State.Set("tag", overVer.Tag())
					sctx.State.Set("bump", "override")
				} else {
					next := current.Bump(analysis.Bump)
					sctx.State.Set("tag", next.Tag())
					sctx.State.Set("bump", analysis.Bump)
				}

				log.Printf("next version: %s (bump: %s)", sctx.State.Get("tag"), sctx.State.Get("bump"))

				// Update manifest for Python/Node ecosystems.
				eco := sctx.State.Get("ecosystem")
				manifest := sctx.State.Get("manifest-path")
				if manifest != "" && (eco == "python" || eco == "node") {
					tag := sctx.State.Get("tag")
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
			Run: func(sctx *pipeline.StepContext) error {
				notes := sctx.State.Get("notes")
				if notes == "" {
					return fmt.Errorf("no release notes in state — run bump step first")
				}
				log.Printf("release notes generated (%d chars)", len(notes))
				return nil
			},
		},
		{
			Name: "release",
			Run: func(sctx *pipeline.StepContext) error {
				tag := sctx.State.Get("tag")
				notes := sctx.State.Get("notes")
				eco := sctx.State.Get("ecosystem")
				hasGoreleaser := sctx.State.Get("has-goreleaser") == "true"

				if opts.DryRun {
					log.Printf("[dry-run] would create release %s", tag)
					log.Printf("[dry-run] release notes:\n%s", notes)
					return nil
				}

				// Skip tag creation in CI mode (--tag): tag already exists
				if opts.Tag == "" {
					// For Python/Node, commit and push the manifest change.
					if eco == "python" || eco == "node" {
						if err := git.CommitAll(ctx, dir, fmt.Sprintf("release: %s", tag)); err != nil {
							return fmt.Errorf("commit manifest: %w", err)
						}
						if err := git.Push(ctx, dir); err != nil {
							return fmt.Errorf("push manifest commit: %w", err)
						}
					}

					if err := git.CreateTag(ctx, dir, tag); err != nil {
						return fmt.Errorf("create tag: %w", err)
					}
					if err := git.PushTag(ctx, dir, tag); err != nil {
						return fmt.Errorf("push tag: %w", err)
					}
				}

				if hasGoreleaser {
					if err := runGoreleaser(); err != nil {
						return fmt.Errorf("goreleaser: %w", err)
					}
					if err := ghClient.EditReleaseBody(ctx, owner, repo, tag, notes); err != nil {
						return fmt.Errorf("edit release body: %w", err)
					}
				} else {
					releaseURL, err := ghClient.CreateRelease(ctx, owner, repo, github.ReleaseParams{
						Tag:        tag,
						Name:       tag,
						Body:       notes,
						Draft:      cfg.GitHub.Draft,
						Prerelease: cfg.GitHub.Prerelease,
					})
					if err != nil {
						return fmt.Errorf("create release: %w", err)
					}
					sctx.State.Set("release-url", releaseURL)
					log.Printf("release created: %s", releaseURL)
				}

				return nil
			},
		},
		{
			Name: "sign",
			Run: func(sctx *pipeline.StepContext) error {
				tag := sctx.State.Get("tag")
				if opts.DryRun {
					log.Printf("[dry-run] would sign %s", tag)
					return nil
				}
				return sign.Run(ctx, opts.Sign, tag, owner, repo)
			},
		},
	}

	p := pipeline.New(statePath, steps)

	if opts.Step != "" {
		return p.RunStep(ctx, opts.Step, opts.Force)
	}
	return p.Run(ctx, opts.Force)
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
