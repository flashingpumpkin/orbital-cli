// Package workflow provides workflow configuration and preset management for orbit.
package workflow

import (
	"errors"
	"fmt"
)

// Step represents a single step in a workflow.
type Step struct {
	// Name is the unique identifier for this step (required).
	Name string `toml:"name" json:"name"`

	// Prompt is the prompt sent to Claude for this step (required).
	Prompt string `toml:"prompt" json:"prompt"`

	// Gate marks this step as a quality gate that must pass before continuing.
	Gate bool `toml:"gate" json:"gate,omitempty"`

	// OnFail specifies the step name to return to if this gate fails.
	OnFail string `toml:"on_fail" json:"on_fail,omitempty"`

	// Deferred marks this step to be skipped during normal execution.
	// Deferred steps only run when reached via a gate's OnFail jump.
	Deferred bool `toml:"deferred" json:"deferred,omitempty"`
}

// Workflow represents a multi-step workflow configuration.
type Workflow struct {
	// Name is an optional identifier for custom workflows.
	Name string `toml:"name" json:"name,omitempty"`

	// Preset is the name of a preset workflow to use.
	Preset string `toml:"preset" json:"preset,omitempty"`

	// Steps defines the ordered list of workflow steps.
	Steps []Step `toml:"steps" json:"steps"`

	// MaxGateRetries is the maximum number of times a gate can fail before aborting (default: 3).
	MaxGateRetries int `toml:"max_gate_retries" json:"max_gate_retries,omitempty"`
}

// Validate checks that the workflow configuration is valid.
func (w *Workflow) Validate() error {
	if len(w.Steps) == 0 && w.Preset == "" {
		return errors.New("workflow must have at least one step or specify a preset")
	}

	stepNames := make(map[string]bool)
	for i, step := range w.Steps {
		if step.Name == "" {
			return fmt.Errorf("step %d: name is required", i+1)
		}
		if step.Prompt == "" {
			return fmt.Errorf("step %d (%s): prompt is required", i+1, step.Name)
		}
		if stepNames[step.Name] {
			return fmt.Errorf("step %d: duplicate step name %q", i+1, step.Name)
		}
		stepNames[step.Name] = true

		if step.OnFail != "" && !step.Gate {
			return fmt.Errorf("step %d (%s): on_fail requires gate = true", i+1, step.Name)
		}
	}

	// Validate on_fail references existing steps
	for i, step := range w.Steps {
		if step.OnFail != "" {
			if !stepNames[step.OnFail] {
				return fmt.Errorf("step %d (%s): on_fail references unknown step %q", i+1, step.Name, step.OnFail)
			}
		}
	}

	// Validate deferred steps are reachable via OnFail
	onFailTargets := make(map[string]bool)
	for _, step := range w.Steps {
		if step.OnFail != "" {
			onFailTargets[step.OnFail] = true
		}
	}
	for i, step := range w.Steps {
		if step.Deferred && !onFailTargets[step.Name] {
			return fmt.Errorf("step %d (%s): deferred step is unreachable (not targeted by any on_fail)", i+1, step.Name)
		}
	}

	return nil
}

// GetStepIndex returns the index of the step with the given name, or -1 if not found.
func (w *Workflow) GetStepIndex(name string) int {
	for i, step := range w.Steps {
		if step.Name == name {
			return i
		}
	}
	return -1
}

// DefaultMaxGateRetries is the default maximum number of gate retries.
const DefaultMaxGateRetries = 3

// EffectiveMaxGateRetries returns the configured or default max gate retries.
func (w *Workflow) EffectiveMaxGateRetries() int {
	if w.MaxGateRetries > 0 {
		return w.MaxGateRetries
	}
	return DefaultMaxGateRetries
}

// HasGates returns true if any step in the workflow is a gate.
func (w *Workflow) HasGates() bool {
	for _, step := range w.Steps {
		if step.Gate {
			return true
		}
	}
	return false
}
