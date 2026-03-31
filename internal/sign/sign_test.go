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
	ctx := t.Context()
	err := sign.Run(ctx, false, "v1.0.0", "owner", "repo")
	if err != nil {
		t.Errorf("expected nil when signing disabled, got: %v", err)
	}
}

func TestCosignArgsFormat(t *testing.T) {
	args := sign.CosignArgs("v1.0.0", "myowner", "myrepo")
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(args))
	}
	if args[0] != "sign" {
		t.Errorf("args[0]: got %s, want sign", args[0])
	}
	if args[1] != "--yes" {
		t.Errorf("args[1]: got %s, want --yes", args[1])
	}
	expected := "ghcr.io/myowner/myrepo:v1.0.0"
	if args[2] != expected {
		t.Errorf("args[2]: got %s, want %s", args[2], expected)
	}
}
