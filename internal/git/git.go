package git

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Commit struct {
	Hash    string
	Subject string
}

func LatestTag(dir string) (string, error) {
	out, err := runGit(dir, "tag", "--sort=-version:refname")
	if err != nil {
		return "", fmt.Errorf("no tags found: %w", err)
	}
	tags := strings.Split(strings.TrimSpace(out), "\n")
	if len(tags) == 0 || tags[0] == "" {
		return "", fmt.Errorf("no tags found")
	}
	return tags[0], nil
}

func CommitsSince(dir string, tag string) ([]Commit, error) {
	out, err := runGit(dir, "log", tag+"..HEAD", "--pretty=format:%H %s")
	if err != nil {
		return nil, fmt.Errorf("git log since %s: %w", tag, err)
	}

	if strings.TrimSpace(out) == "" {
		return nil, nil
	}

	var commits []Commit
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		commits = append(commits, Commit{Hash: parts[0], Subject: parts[1]})
	}
	return commits, nil
}

func CreateTag(dir string, tag string) error {
	_, err := runGit(dir, "tag", "-a", tag, "-m", tag)
	if err != nil {
		return fmt.Errorf("create tag %s: %w", tag, err)
	}
	return nil
}

func PushTag(dir string, tag string) error {
	_, err := runGit(dir, "push", "origin", tag)
	if err != nil {
		return fmt.Errorf("push tag %s: %w", tag, err)
	}
	return nil
}

func CommitAll(dir string, message string) error {
	if _, err := runGit(dir, "add", "-A"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	if _, err := runGit(dir, "commit", "-m", message); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	return nil
}

func Push(dir string) error {
	_, err := runGit(dir, "push")
	if err != nil {
		return fmt.Errorf("git push: %w", err)
	}
	return nil
}

func RemoteURL(dir string) (string, error) {
	out, err := runGit(dir, "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("get remote URL: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func TagTimestamp(dir string, tag string) (time.Time, error) {
	out, err := runGit(dir, "log", "-1", "--format=%aI", tag)
	if err != nil {
		return time.Time{}, fmt.Errorf("get tag timestamp for %s: %w", tag, err)
	}
	return time.Parse(time.RFC3339, strings.TrimSpace(out))
}

func DiffSince(dir string, tag string) (string, error) {
	out, err := runGit(dir, "diff", tag+"..HEAD")
	if err != nil {
		return "", fmt.Errorf("diff since %s: %w", tag, err)
	}
	return out, nil
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
