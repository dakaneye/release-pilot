package config

import (
	"errors"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all release-pilot configuration.
type Config struct {
	Ecosystem string       `yaml:"ecosystem"`
	Model     string       `yaml:"model"`
	TagPrefix string       `yaml:"tag-prefix"`
	SubDir    string       `yaml:"sub-dir"`
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
// then applies RELEASE_PILOT_MODEL env var override if set. Missing files return
// defaults with nil error; YAML parse failures return an error.
func Load(path string) (Config, error) {
	cfg := defaults()

	if path == "" {
		path = ".release-pilot.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return applyEnv(cfg), nil
		}
		return cfg, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	return applyEnv(cfg), nil
}

func applyEnv(cfg Config) Config {
	if env := os.Getenv("RELEASE_PILOT_MODEL"); env != "" {
		cfg.Model = env
	}
	return cfg
}
