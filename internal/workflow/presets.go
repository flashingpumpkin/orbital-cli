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

// rigorousReviewPrompt is the shared review prompt used across all presets with review gates.
const rigorousReviewPrompt = `You are conducting a rigorous code review. Your job is to FIND PROBLEMS, not to rubber-stamp code.

First, identify all files changed in this iteration using git diff.

Then spawn FIVE parallel review agents using the Task tool. Each agent reviews the SAME changed files but for different concerns:

1. security-reviewer - vulnerabilities, injection, auth issues
2. design-reviewer - architecture, SOLID, coupling
3. logic-reviewer - bugs, edge cases, race conditions
4. error-reviewer - exception safety, recovery, logging
5. data-reviewer - validation, consistency, null safety

Wait for ALL agents to complete. Collect their outputs.

Aggregate the results in the notes file with this structure:
## Code Review - Iteration N

### Security
[security-reviewer findings or "No issues"]

### Design
[design-reviewer findings or "No issues"]

### Logic
[logic-reviewer findings or "No issues"]

### Error Handling
[error-reviewer findings or "No issues"]

### Data Integrity
[data-reviewer findings or "No issues"]

### Verdict
[PASS or FAIL with summary]

GATE DECISION:
- If ANY agent found issues (output contains _ISSUES_FOUND), output <gate>FAIL</gate>
- ONLY if ALL agents output _CLEAR, output <gate>PASS</gate>

Be ruthless. A PASS means you are confident this code is production-ready.`

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
				Name:   "review",
				Prompt: rigorousReviewPrompt,
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
				Name:   "review",
				Prompt: rigorousReviewPrompt,
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
				Name:   "review",
				Prompt: rigorousReviewPrompt,
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
