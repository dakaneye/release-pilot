package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all release-pilot configuration.
type Config struct {
	Ecosystem string       `yaml:"ecosystem"`
	Model     string       `yaml:"model"`
	Notes     NotesConfig  `yaml:"notes"`
	GitHub    GitHubConfig `yaml:"github"`
}

// NotesConfig controls release note generation behavior.
type NotesConfig struct {
	IncludeDiffs bool `yaml:"include-diffs"`
}

// GitHubConfig controls GitHub release creation behavior.
type GitHubConfig struct {
	Draft      bool `yaml:"draft"`
	Prerelease bool `yaml:"prerelease"`
}

func defaults() Config {
	return Config{
		Ecosystem: "auto",
		Model:     "claude-sonnet-4-6",
	}
}

// Load reads config from path (falling back to ".release-pilot.yaml" if empty),
// then applies RELEASE_PILOT_MODEL env var override if set. Missing files are
// silently ignored and defaults are used.
func Load(path string) Config {
	cfg := defaults()

	if path == "" {
		path = ".release-pilot.yaml"
	}

	data, err := os.ReadFile(path)
	if err == nil {
		_ = yaml.Unmarshal(data, &cfg)
	}

	if env := os.Getenv("RELEASE_PILOT_MODEL"); env != "" {
		cfg.Model = env
	}

	return cfg
}
