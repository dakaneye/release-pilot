package sign

import (
	"context"
	"fmt"
	"os/exec"
)

func CosignArgs(tag, owner, repo string) []string {
	return []string{
		"sign",
		"--yes",
		fmt.Sprintf("ghcr.io/%s/%s:%s", owner, repo, tag),
	}
}

func Run(ctx context.Context, enabled bool, tag, owner, repo string) error {
	if !enabled {
		return nil
	}

	args := CosignArgs(tag, owner, repo)
	cmd := exec.CommandContext(ctx, "cosign", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cosign sign: %w\n%s", err, string(out))
	}
	return nil
}
