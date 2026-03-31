package version

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Semver holds major, minor, and patch version components.
type Semver struct {
	Major int
	Minor int
	Patch int
}

// String returns the dotted version string without a leading "v".
func (v Semver) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Tag returns the version string with a leading "v" for use as a git tag.
func (v Semver) Tag() string {
	return "v" + v.String()
}

// Bump returns a new Semver incremented at the given level.
// Unknown levels return the receiver unchanged.
func (v Semver) Bump(level string) Semver {
	switch level {
	case "major":
		return Semver{Major: v.Major + 1}
	case "minor":
		return Semver{Major: v.Major, Minor: v.Minor + 1}
	case "patch":
		return Semver{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1}
	default:
		return v
	}
}

var tagPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

// ParseTag parses a semver tag with an optional leading "v".
func ParseTag(tag string) (Semver, error) {
	matches := tagPattern.FindStringSubmatch(tag)
	if matches == nil {
		return Semver{}, fmt.Errorf("invalid semver tag: %s", tag)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	return Semver{Major: major, Minor: minor, Patch: patch}, nil
}

// PrefixedTag returns the version as a git tag with the given prefix.
// When prefix is empty, this is equivalent to Tag().
func (v Semver) PrefixedTag(prefix string) string {
	return prefix + "v" + v.String()
}

// ParsePrefixedTag strips the given prefix from a tag and parses the remainder as semver.
// When prefix is empty, this is equivalent to ParseTag.
// Returns an error if the tag does not start with the expected prefix.
func ParsePrefixedTag(tag string, prefix string) (Semver, error) {
	if prefix != "" {
		if !strings.HasPrefix(tag, prefix) {
			return Semver{}, fmt.Errorf("tag %s does not match prefix %s", tag, prefix)
		}
		tag = strings.TrimPrefix(tag, prefix)
	}
	return ParseTag(tag)
}

// UpdateManifest updates the version field in a project manifest file.
// Supported ecosystems: "node" (package.json), "python" (pyproject.toml).
// Unknown ecosystems are silently ignored.
func UpdateManifest(path string, ecosystem string, newVersion string) error {
	switch ecosystem {
	case "node":
		return updateJSON(path, newVersion)
	case "python":
		return updateTOML(path, newVersion)
	default:
		return nil
	}
}

var jsonVersionPattern = regexp.MustCompile(`("version"\s*:\s*")([^"]+)(")`)

func updateJSON(path string, newVersion string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	if !jsonVersionPattern.Match(data) {
		return fmt.Errorf("no version field found in %s", path)
	}

	updated := jsonVersionPattern.ReplaceAll(data, []byte("${1}"+newVersion+"${3}"))
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	// Update package-lock.json if present alongside package.json.
	dir := filepath.Dir(path)
	lockPath := filepath.Join(dir, "package-lock.json")
	if lockData, err := os.ReadFile(lockPath); err == nil {
		if jsonVersionPattern.Match(lockData) {
			lockUpdated := jsonVersionPattern.ReplaceAll(lockData, []byte("${1}"+newVersion+"${3}"))
			if err := os.WriteFile(lockPath, lockUpdated, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", lockPath, err)
			}
		}
	}

	return nil
}

var tomlVersionPattern = regexp.MustCompile(`(?m)^(version\s*=\s*")([^"]+)(")`)

func updateTOML(path string, newVersion string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	content := string(data)
	if !tomlVersionPattern.MatchString(content) {
		return fmt.Errorf("no version field found in %s", path)
	}

	updated := tomlVersionPattern.ReplaceAllString(content, "${1}"+newVersion+"${3}")

	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}
