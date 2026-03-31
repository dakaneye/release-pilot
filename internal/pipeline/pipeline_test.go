package pipeline_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/dakaneye/release-pilot/internal/pipeline"
)

func TestPipelineRunsAllSteps(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, ".release-pilot-state.json")

	var executed []string
	makeStep := func(name string) pipeline.Step {
		return pipeline.Step{
			Name: name,
			Run: func(ctx *pipeline.Context) error {
				executed = append(executed, name)
				return nil
			},
		}
	}

	p := pipeline.New(statePath, []pipeline.Step{
		makeStep("detect"),
		makeStep("bump"),
		makeStep("notes"),
		makeStep("release"),
		makeStep("sign"),
	})

	if err := p.Run(false); err != nil {
		t.Fatal(err)
	}

	expected := []string{"detect", "bump", "notes", "release", "sign"}
	if len(executed) != len(expected) {
		t.Fatalf("expected %d steps, got %d", len(expected), len(executed))
	}
	for i, name := range expected {
		if executed[i] != name {
			t.Errorf("step %d: expected %s, got %s", i, name, executed[i])
		}
	}
}

func TestPipelineSkipsCompletedSteps(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, ".release-pilot-state.json")

	state, _ := pipeline.LoadState(statePath)
	state.Complete("detect")
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}

	var executed []string
	makeStep := func(name string) pipeline.Step {
		return pipeline.Step{
			Name: name,
			Run: func(ctx *pipeline.Context) error {
				executed = append(executed, name)
				return nil
			},
		}
	}

	p := pipeline.New(statePath, []pipeline.Step{
		makeStep("detect"),
		makeStep("bump"),
	})

	if err := p.Run(false); err != nil {
		t.Fatal(err)
	}

	if len(executed) != 1 || executed[0] != "bump" {
		t.Errorf("expected only bump to run, got %v", executed)
	}
}

func TestPipelineForceResetsState(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, ".release-pilot-state.json")

	state, _ := pipeline.LoadState(statePath)
	state.Complete("detect")
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}

	var executed []string
	makeStep := func(name string) pipeline.Step {
		return pipeline.Step{
			Name: name,
			Run: func(ctx *pipeline.Context) error {
				executed = append(executed, name)
				return nil
			},
		}
	}

	p := pipeline.New(statePath, []pipeline.Step{
		makeStep("detect"),
		makeStep("bump"),
	})

	if err := p.Run(true); err != nil {
		t.Fatal(err)
	}

	if len(executed) != 2 {
		t.Errorf("expected 2 steps with force, got %d: %v", len(executed), executed)
	}
}

func TestPipelineStopsOnError(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, ".release-pilot-state.json")

	var executed []string
	p := pipeline.New(statePath, []pipeline.Step{
		{Name: "detect", Run: func(ctx *pipeline.Context) error {
			executed = append(executed, "detect")
			return nil
		}},
		{Name: "bump", Run: func(ctx *pipeline.Context) error {
			executed = append(executed, "bump")
			return fmt.Errorf("API key missing")
		}},
		{Name: "notes", Run: func(ctx *pipeline.Context) error {
			executed = append(executed, "notes")
			return nil
		}},
	})

	err := p.Run(false)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(executed) != 2 {
		t.Errorf("expected 2 steps before error, got %d: %v", len(executed), executed)
	}

	state, _ := pipeline.LoadState(statePath)
	if !state.IsCompleted("detect") {
		t.Error("detect should be completed")
	}
	if state.IsCompleted("bump") {
		t.Error("bump should not be completed (it failed)")
	}
}

func TestPipelineRunSingleStep(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, ".release-pilot-state.json")

	var executed []string
	makeStep := func(name string) pipeline.Step {
		return pipeline.Step{
			Name: name,
			Run: func(ctx *pipeline.Context) error {
				executed = append(executed, name)
				return nil
			},
		}
	}

	p := pipeline.New(statePath, []pipeline.Step{
		makeStep("detect"),
		makeStep("bump"),
		makeStep("notes"),
	})

	if err := p.RunStep("bump", false); err != nil {
		t.Fatal(err)
	}

	if len(executed) != 1 || executed[0] != "bump" {
		t.Errorf("expected only bump, got %v", executed)
	}
}
