package version_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dakaneye/release-pilot/internal/version"
)

func TestParseTag(t *testing.T) {
	tests := []struct {
		tag     string
		major   int
		minor   int
		patch   int
		wantErr bool
	}{
		{"v1.2.3", 1, 2, 3, false},
		{"1.2.3", 1, 2, 3, false},
		{"v0.0.1", 0, 0, 1, false},
		{"invalid", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			v, err := version.ParseTag(tt.tag)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if v.Major != tt.major || v.Minor != tt.minor || v.Patch != tt.patch {
				t.Errorf("expected %d.%d.%d, got %d.%d.%d", tt.major, tt.minor, tt.patch, v.Major, v.Minor, v.Patch)
			}
		})
	}
}

func TestBump(t *testing.T) {
	v := version.Semver{Major: 1, Minor: 2, Patch: 3}

	major := v.Bump("major")
	if major.String() != "2.0.0" {
		t.Errorf("major bump: expected 2.0.0, got %s", major.String())
	}

	minor := v.Bump("minor")
	if minor.String() != "1.3.0" {
		t.Errorf("minor bump: expected 1.3.0, got %s", minor.String())
	}

	patch := v.Bump("patch")
	if patch.String() != "1.2.4" {
		t.Errorf("patch bump: expected 1.2.4, got %s", patch.String())
	}
}

func TestBumpTag(t *testing.T) {
	v := version.Semver{Major: 1, Minor: 2, Patch: 3}
	bumped := v.Bump("minor")
	if bumped.Tag() != "v1.3.0" {
		t.Errorf("expected v1.3.0, got %s", bumped.Tag())
	}
}

func TestUpdatePackageJSON(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "package.json")
	if err := os.WriteFile(pkg, []byte(`{
  "name": "my-app",
  "version": "1.0.0",
  "description": "test"
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	err := version.UpdateManifest(pkg, "node", "1.1.0")
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(pkg)
	content := string(data)
	if !strings.Contains(content, `"version": "1.1.0"`) {
		t.Errorf("expected version 1.1.0 in:\n%s", content)
	}
	if !strings.Contains(content, `"name": "my-app"`) {
		t.Error("other fields should be preserved")
	}
}

func TestUpdatePackageJSONPreservesKeyOrder(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "package.json")
	original := `{
  "name": "my-app",
  "version": "1.0.0",
  "description": "test",
  "main": "index.js"
}`
	if err := os.WriteFile(pkg, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := version.UpdateManifest(pkg, "node", "2.0.0"); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(pkg)
	content := string(data)

	nameIdx := strings.Index(content, `"name"`)
	versionIdx := strings.Index(content, `"version"`)
	descIdx := strings.Index(content, `"description"`)
	mainIdx := strings.Index(content, `"main"`)

	if nameIdx >= versionIdx {
		t.Errorf("name should appear before version in output:\n%s", content)
	}
	if versionIdx >= descIdx {
		t.Errorf("version should appear before description in output:\n%s", content)
	}
	if descIdx >= mainIdx {
		t.Errorf("description should appear before main in output:\n%s", content)
	}
	if !strings.Contains(content, `"version": "2.0.0"`) {
		t.Errorf("expected version 2.0.0 in:\n%s", content)
	}
}

func TestUpdatePyprojectTOML(t *testing.T) {
	dir := t.TempDir()
	pyproject := filepath.Join(dir, "pyproject.toml")
	if err := os.WriteFile(pyproject, []byte(`[project]
name = "my-app"
version = "1.0.0"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	err := version.UpdateManifest(pyproject, "python", "1.1.0")
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(pyproject)
	content := string(data)
	if !strings.Contains(content, `version = "1.1.0"`) {
		t.Errorf("expected version 1.1.0 in:\n%s", content)
	}
}

func TestParsePrefixedTag(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		prefix  string
		major   int
		minor   int
		patch   int
		wantErr bool
	}{
		{"standard prefix", "review-code/v1.2.3", "review-code/", 1, 2, 3, false},
		{"prefix without v", "review-code/1.2.3", "review-code/", 1, 2, 3, false},
		{"empty prefix passthrough", "v1.2.3", "", 1, 2, 3, false},
		{"wrong prefix", "other-code/v1.2.3", "review-code/", 0, 0, 0, true},
		{"no prefix on prefixed tag", "v1.2.3", "review-code/", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := version.ParsePrefixedTag(tt.tag, tt.prefix)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if v.Major != tt.major || v.Minor != tt.minor || v.Patch != tt.patch {
				t.Errorf("expected %d.%d.%d, got %d.%d.%d", tt.major, tt.minor, tt.patch, v.Major, v.Minor, v.Patch)
			}
		})
	}
}

func TestPrefixedTag(t *testing.T) {
	v := version.Semver{Major: 1, Minor: 2, Patch: 3}

	if got := v.PrefixedTag("review-code/"); got != "review-code/v1.2.3" {
		t.Errorf("expected review-code/v1.2.3, got %s", got)
	}
	if got := v.PrefixedTag(""); got != "v1.2.3" {
		t.Errorf("expected v1.2.3, got %s", got)
	}
}

func TestUpdatePackageJSONWithLockfile(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "package.json")
	lock := filepath.Join(dir, "package-lock.json")
	if err := os.WriteFile(pkg, []byte(`{
  "name": "my-app",
  "version": "1.0.0"
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lock, []byte(`{
  "name": "my-app",
  "version": "1.0.0",
  "lockfileVersion": 3
}`), 0o644); err != nil {
		t.Fatal(err)
	}

	err := version.UpdateManifest(pkg, "node", "1.1.0")
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(lock)
	content := string(data)
	if !strings.Contains(content, `"version": "1.1.0"`) {
		t.Errorf("expected version 1.1.0 in lock file:\n%s", content)
	}
}
