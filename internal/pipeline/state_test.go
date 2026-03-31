package pipeline_test

import (
	"path/filepath"
	"testing"

	"github.com/dakaneye/release-pilot/internal/pipeline"
)

func TestNewState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".release-pilot-state.json")

	state, err := pipeline.LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if state.IsCompleted("detect") {
		t.Error("detect should not be completed in fresh state")
	}
}

func TestCompleteStep(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".release-pilot-state.json")

	state, _ := pipeline.LoadState(path)
	state.Complete("detect")
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := pipeline.LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.IsCompleted("detect") {
		t.Error("detect should be completed after reload")
	}
	if reloaded.IsCompleted("bump") {
		t.Error("bump should not be completed")
	}
}

func TestStateStoresData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".release-pilot-state.json")

	state, _ := pipeline.LoadState(path)
	state.Set("tag", "v1.2.0")
	state.Set("notes", "## Features\n- stuff")
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, _ := pipeline.LoadState(path)
	if reloaded.Get("tag") != "v1.2.0" {
		t.Errorf("expected v1.2.0, got %s", reloaded.Get("tag"))
	}
	if reloaded.Get("notes") == "" {
		t.Error("expected notes to be stored")
	}
}

func TestReset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".release-pilot-state.json")

	state, _ := pipeline.LoadState(path)
	state.Complete("detect")
	state.Set("tag", "v1.0.0")
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}

	state.Reset()
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, _ := pipeline.LoadState(path)
	if reloaded.IsCompleted("detect") {
		t.Error("detect should not be completed after reset")
	}
	if reloaded.Get("tag") != "" {
		t.Error("tag should be empty after reset")
	}
}
