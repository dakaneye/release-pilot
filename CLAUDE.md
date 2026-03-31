# Releases

All dakaneye repos use [release-pilot](https://github.com/dakaneye/release-pilot) for releases.

## Cutting a release

When asked to cut, ship, or create a release:

1. Ensure all tests pass (`go test ./...`, `npm test`, `pytest`, etc. depending on ecosystem)
2. Run `release-pilot ship --dry-run` first to preview the version bump and release notes
3. If the dry-run looks good, create and push the tag — the release workflow handles the rest:
   ```bash
   git tag -a v<version> -m "v<version>"
   git push origin v<version>
   ```
4. Watch the release workflow: `gh run watch $(gh run list --limit 1 --json databaseId --jq '.[0].databaseId')`
5. Verify the release: `gh release view v<version>`

Do NOT run `release-pilot ship` directly in CI-less repos. The release workflow in CI runs it with the `--tag` flag after the tag is pushed.

## Monorepo releases

For monorepos with independently versioned sub-projects (e.g., claude-skills):

- Tags use `<name>/v<semver>` format (e.g., `review-code/v1.0.5`)
- Configure `.release-pilot.yaml` with `tag-prefix` and `sub-dir`
- Push prefixed tags: `git push origin review-code/v1.0.5`

## Setting up a new repo

When a repo needs release-pilot for the first time:

1. Run `release-pilot init` to scaffold `.release-pilot.yaml`
2. Copy the release workflow from release-pilot's repo:
   ```bash
   gh api repos/dakaneye/release-pilot/contents/.github/workflows/release.yml --jq '.content' | base64 -d > .github/workflows/release.yml
   ```
3. Adjust the workflow if the repo doesn't use goreleaser (remove the goreleaser install step)
4. Create an initial tag: `git tag -a v0.1.0 -m "v0.1.0" && git push origin v0.1.0`
5. Ensure `ANTHROPIC_API_KEY` is set in the repo's GitHub Actions secrets

## Requirements

- `ANTHROPIC_API_KEY` secret in GitHub Actions
- `GITHUB_TOKEN` is auto-provided by GitHub Actions
- A release workflow triggered on tag push (`on: push: tags: ["v*"]`)
