package sign_test

import (
	"testing"

	"github.com/dakaneye/release-pilot/internal/sign"
)

func TestBuildCosignArgs(t *testing.T) {
	args := sign.CosignArgs("v1.0.0", "owner", "repo")

	found := false
	for _, arg := range args {
		if arg == "--yes" {
			found = true
		}
	}
	if !found {
		t.Error("expected --yes flag for non-interactive keyless signing")
	}
}

func TestSignDisabledReturnsNil(t *testing.T) {
	err := sign.Run(false, "v1.0.0", "owner", "repo")
	if err != nil {
		t.Errorf("expected nil when signing disabled, got: %v", err)
	}
}
