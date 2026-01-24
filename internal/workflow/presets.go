package workflow

import "fmt"

// PresetName represents a valid preset workflow name.
type PresetName string

const (
	// PresetFast tries to complete all work in a single iteration.
	PresetFast PresetName = "fast"

	// PresetSpecDriven is the default preset with a single implement step.
	PresetSpecDriven PresetName = "spec-driven"

	// PresetReviewed adds a review gate after implementation.
	PresetReviewed PresetName = "reviewed"

	// PresetTDD implements test-driven development with red-green-refactor cycle.
	PresetTDD PresetName = "tdd"
)

// DefaultPreset is the preset used when none is specified.
const DefaultPreset = PresetSpecDriven

// ValidPresets returns all valid preset names.
func ValidPresets() []PresetName {
	return []PresetName{PresetFast, PresetSpecDriven, PresetReviewed, PresetTDD}
}

// IsValidPreset returns true if the given name is a valid preset.
func IsValidPreset(name string) bool {
	for _, p := range ValidPresets() {
		if string(p) == name {
			return true
		}
	}
	return false
}

// GetPreset returns the workflow configuration for the given preset name.
// Returns an error if the preset name is invalid.
func GetPreset(name PresetName) (*Workflow, error) {
	switch name {
	case PresetFast:
		return fastPreset(), nil
	case PresetSpecDriven:
		return specDrivenPreset(), nil
	case PresetReviewed:
		return reviewedPreset(), nil
	case PresetTDD:
		return tddPreset(), nil
	default:
		return nil, fmt.Errorf("unknown preset: %s", name)
	}
}

// fastPreset returns the fast workflow that maximises work per iteration.
func fastPreset() *Workflow {
	return &Workflow{
		Name:   string(PresetFast),
		Preset: string(PresetFast),
		Steps: []Step{
			{
				Name: "implement",
				Prompt: `Implement as many requirements as possible from {{files}} in this iteration.

Do not work incrementally. Tackle multiple requirements at once:
1. Read all remaining requirements
2. Implement as many as you can with tests
3. Run tests to verify everything works
4. Check off completed items in the spec file

Maximise throughput. Do not stop after one item.
Do not output completion promise yet.`,
			},
			{
				Name: "review",
				Prompt: `Review all code changes made in this iteration.
Check for: correctness, edge cases, code quality, test coverage.

Write your findings to the notes file with clear action items if any issues found.

If changes are acceptable with no blocking issues, output <gate>PASS</gate>
If changes need work, output <gate>FAIL</gate>`,
				Gate:   true,
				OnFail: "implement",
			},
		},
	}
}

// specDrivenPreset returns the spec-driven workflow (default).
func specDrivenPreset() *Workflow {
	return &Workflow{
		Name:   string(PresetSpecDriven),
		Preset: string(PresetSpecDriven),
		Steps: []Step{
			{
				Name: "implement",
				Prompt: `Read the notes file for any pending feedback to address.
Continue implementing the requirements in {{files}}.
Focus on the next incomplete item.
When all requirements are complete, output <promise>COMPLETE</promise>`,
			},
		},
	}
}

// reviewedPreset returns the reviewed workflow with a review gate.
func reviewedPreset() *Workflow {
	return &Workflow{
		Name:   string(PresetReviewed),
		Preset: string(PresetReviewed),
		Steps: []Step{
			{
				Name: "implement",
				Prompt: `Read the notes file for any review feedback to address.
If there is feedback, address it first before continuing.
Then continue implementing the requirements in {{files}}.
Focus on the next incomplete item.
Do not output completion promise yet.`,
			},
			{
				Name: "review",
				Prompt: `Review the code changes just made.
Check for: correctness, edge cases, code quality, test coverage.

Write your findings to the notes file with clear action items if any issues found.

If changes are acceptable with no blocking issues, output <gate>PASS</gate>
If changes need work, output <gate>FAIL</gate>`,
				Gate:   true,
				OnFail: "implement",
			},
		},
	}
}

// tddPreset returns the TDD workflow with red-green-refactor cycle.
func tddPreset() *Workflow {
	return &Workflow{
		Name:   string(PresetTDD),
		Preset: string(PresetTDD),
		Steps: []Step{
			{
				Name: "red",
				Prompt: `Read the notes file for any feedback from previous review.
Write a failing test for the next requirement in {{files}}.
The test should fail because the functionality doesn't exist yet.
Run the test to confirm it fails.
Write to notes: what test was added and what it tests.`,
			},
			{
				Name: "green",
				Prompt: `Read the notes file to understand what test was just written.
Write the minimal code to make the failing test pass.
Do not add extra functionality beyond what the test requires.
Run the test to confirm it passes.
Update notes: what implementation was added.`,
			},
			{
				Name: "refactor",
				Prompt: `Read the notes file to understand the current TDD cycle.
If there is review feedback from a previous failed gate, address it first.
Refactor the code while keeping tests green.
Improve clarity, remove duplication, apply good design principles.
Run tests to confirm they still pass.
Update notes: what refactoring was done.`,
			},
			{
				Name: "review",
				Prompt: `Read the notes file to understand the TDD cycle just completed.
Review: test quality, implementation correctness, refactoring quality.

Write detailed findings to the notes file.
If issues found, write clear action items for the refactor step.

If acceptable, output <gate>PASS</gate>
If needs work, output <gate>FAIL</gate>`,
				Gate:   true,
				OnFail: "refactor",
			},
		},
	}
}

// PresetDescriptions returns brief descriptions for each preset.
func PresetDescriptions() map[PresetName]string {
	return map[PresetName]string{
		PresetFast:       "Maximise work per iteration with review gate",
		PresetSpecDriven: "Single implement step with completion check (default)",
		PresetReviewed:   "Implement with review gate before completion",
		PresetTDD:        "Red-green-refactor cycle with review gate",
	}
}
