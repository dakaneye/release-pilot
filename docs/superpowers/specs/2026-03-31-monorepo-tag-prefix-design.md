# Monorepo Tag Prefix Support

Implements [#6](https://github.com/dakaneye/release-pilot/issues/6).

## Problem

release-pilot assumes a single project per repo with plain `vX.Y.Z` tags. Monorepos use prefixed tags like `review-code/v1.0.5` to independently version sub-projects.

## Config

Two new optional fields in `.release-pilot.yaml`:

```yaml
ecosystem: python
tag-prefix: review-code/
sub-dir: review-code/

notes:
  include-diffs: false
```

- `tag-prefix` (string, default `""`) — prepended to `v<semver>` when creating and filtering tags. Example: `review-code/` produces tags like `review-code/v1.0.5`.
- `sub-dir` (string, default `""`) — restricts `CommitsSince` and `DiffSince` to changes within this directory. Independent of `tag-prefix`.

Both empty = current single-project behavior, fully backward compatible.

## Scope

Only the `<name>/v<semver>` convention (Go modules, most monorepos). Other conventions (`@scope/name@semver`, `name-vsemver`) are future work.

## Package Changes

### `internal/config`

Add `TagPrefix string` and `SubDir string` to `Config` struct.

### `internal/version`

- `ParsePrefixedTag(tag, prefix string) (Semver, error)` — strips prefix via `strings.TrimPrefix`, delegates to `ParseTag`.
- `Semver.PrefixedTag(prefix string) string` — returns `prefix + "v" + semver`. When prefix is empty, equivalent to `Tag()`.

### `internal/git`

Signature changes (backward compatible when prefix/paths are empty):

- `LatestTag(ctx, dir, prefix string)` — runs `git tag --sort=-version:refname`, filters lines starting with prefix, returns first match.
- `PreviousTag(ctx, dir, current, prefix string)` — same filtering, returns tag after current.
- `CommitsSince(ctx, dir, tag string, paths ...string)` — appends `-- <paths>` to `git log` when paths are provided.
- `DiffSince(ctx, dir, tag string, paths ...string)` — appends `-- <paths>` to `git diff` when paths are provided.

### `internal/ship`

Thread `cfg.TagPrefix` and `cfg.SubDir` through the pipeline:

- **bump step**: Use `LatestTag`/`PreviousTag` with prefix. Use `ParsePrefixedTag` to extract semver. Pass `cfg.SubDir` to `CommitsSince`/`DiffSince`. Create new tag with `PrefixedTag(prefix)`.
- **release step**: GitHub release name = prefixed tag (e.g., `review-code/v1.1.0`).

### `cmd/release-pilot/main.go`

Add `--tag-prefix` and `--sub-dir` CLI flags that override config values.

## Behavior Matrix

| Scenario | LatestTag | CommitsSince | CreateTag | Release Name |
|----------|-----------|-------------|-----------|-------------|
| No prefix (default) | First `v*` tag | All commits since tag | `v1.1.0` | `v1.1.0` |
| `tag-prefix: review-code/` | First `review-code/v*` tag | All commits since tag | `review-code/v1.1.0` | `review-code/v1.1.0` |
| prefix + `sub-dir: review-code/` | First `review-code/v*` tag | Commits touching `review-code/` | `review-code/v1.1.0` | `review-code/v1.1.0` |

## Testing

- `ParsePrefixedTag`: valid prefix, wrong prefix, empty prefix, no-v variant
- `PrefixedTag`: with and without prefix
- `LatestTag`/`PreviousTag`: repo with mixed prefixed and unprefixed tags
- `CommitsSince`: with path filtering (commits inside vs. outside subdir)
- `DiffSince`: same path filtering
- Config loading: tag-prefix and sub-dir parsed from YAML
- Ship pipeline: prefix threaded through bump and release steps
- All existing tests pass unchanged (empty prefix = backward compatible)

## Not in Scope

- `@scope/name@semver` or `name-vsemver` conventions
- `MergedPRsSince` path filtering (GitHub API limitation)
- `init` command generating prefix/sub-dir config
- Monorepo usage documentation (follow-up)
