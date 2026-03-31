package pipeline

import (
	"context"
	"fmt"
	"log"
)

type StepContext struct {
	State *State
}

type Step struct {
	Name string
	Run  func(ctx *StepContext) error
}

type Pipeline struct {
	statePath string
	steps     []Step
}

func New(statePath string, steps []Step) *Pipeline {
	return &Pipeline{
		statePath: statePath,
		steps:     steps,
	}
}

func (p *Pipeline) Run(ctx context.Context, force bool) error {
	state, err := LoadState(p.statePath)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	if force {
		state.Reset()
		if err := state.Save(); err != nil {
			return fmt.Errorf("reset state: %w", err)
		}
	}

	stepCtx := &StepContext{State: state}

	for _, step := range p.steps {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("pipeline cancelled: %w", err)
		}

		if state.IsCompleted(step.Name) {
			log.Printf("skipping completed step: %s", step.Name)
			continue
		}

		log.Printf("running step: %s", step.Name)
		if err := step.Run(stepCtx); err != nil {
			return fmt.Errorf("step %s: %w", step.Name, err)
		}

		state.Complete(step.Name)
		if err := state.Save(); err != nil {
			return fmt.Errorf("save state after %s: %w", step.Name, err)
		}
	}

	return nil
}

func (p *Pipeline) RunStep(ctx context.Context, name string, force bool) error {
	state, err := LoadState(p.statePath)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}

	if force {
		delete(state.Steps, name)
	}

	stepCtx := &StepContext{State: state}

	for _, step := range p.steps {
		if step.Name == name {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("pipeline cancelled: %w", err)
			}

			if state.IsCompleted(step.Name) {
				log.Printf("skipping completed step: %s", step.Name)
				return nil
			}

			log.Printf("running step: %s", step.Name)
			if err := step.Run(stepCtx); err != nil {
				return fmt.Errorf("step %s: %w", step.Name, err)
			}

			state.Complete(step.Name)
			return state.Save()
		}
	}

	return fmt.Errorf("unknown step: %s", name)
}
