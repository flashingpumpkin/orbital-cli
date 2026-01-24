package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrMaxGateRetriesExceeded is returned when a gate fails too many times.
var ErrMaxGateRetriesExceeded = errors.New("max gate retries exceeded")

// ExecutionResult contains the result of executing a single step.
type ExecutionResult struct {
	// StepName is the name of the step that was executed.
	StepName string

	// Output is the text output from the step.
	Output string

	// CostUSD is the cost of this step in USD.
	CostUSD float64

	// TokensIn is the number of input tokens used by this step.
	TokensIn int

	// TokensOut is the number of output tokens used by this step.
	TokensOut int
}

// StepExecutor is the interface for executing a single workflow step.
type StepExecutor interface {
	// ExecuteStep executes a single step with the given prompt.
	// Returns the execution result or an error.
	ExecuteStep(ctx context.Context, stepName string, prompt string) (*ExecutionResult, error)
}

// StepInfo provides context about the current step execution.
type StepInfo struct {
	// Name is the step name.
	Name string

	// Position is the 1-indexed position in the workflow.
	Position int

	// Total is the total number of steps in the workflow.
	Total int

	// GateRetries is the current retry count for this step (0 if first attempt).
	GateRetries int

	// MaxRetries is the maximum allowed gate retries.
	MaxRetries int
}

// RunnerCallback is called after each step completes.
type RunnerCallback func(info StepInfo, result *ExecutionResult, gateResult GateResult) error

// StepStartCallback is called before each step begins execution.
type StepStartCallback func(info StepInfo)

// Runner executes a workflow by running its steps in sequence.
type Runner struct {
	workflow      *Workflow
	executor      StepExecutor
	callback      RunnerCallback
	startCallback StepStartCallback

	// filePaths is used for template substitution in prompts.
	filePaths []string
}

// NewRunner creates a new workflow runner.
func NewRunner(w *Workflow, exec StepExecutor) *Runner {
	return &Runner{
		workflow: w,
		executor: exec,
	}
}

// SetCallback sets the callback function called after each step.
func (r *Runner) SetCallback(cb RunnerCallback) {
	r.callback = cb
}

// SetStartCallback sets the callback function called before each step starts.
func (r *Runner) SetStartCallback(cb StepStartCallback) {
	r.startCallback = cb
}

// SetFilePaths sets the file paths for template substitution.
func (r *Runner) SetFilePaths(paths []string) {
	r.filePaths = paths
}

// RunResult contains the result of running the entire workflow.
type RunResult struct {
	// Steps contains results for each step executed.
	Steps []*StepResult

	// TotalCost is the cumulative cost in USD.
	TotalCost float64

	// TotalTokensIn is the cumulative input token count.
	TotalTokensIn int

	// TotalTokensOut is the cumulative output token count.
	TotalTokensOut int

	// CompletedAllSteps is true if all steps completed successfully.
	CompletedAllSteps bool
}

// StepResult contains the result of a single step execution.
type StepResult struct {
	StepName   string
	Output     string
	CostUSD    float64
	TokensIn   int
	TokensOut  int
	GateResult GateResult
	RetryCount int
}

// Run executes all workflow steps in sequence.
// Returns the run result or an error if execution fails.
func (r *Runner) Run(ctx context.Context) (*RunResult, error) {
	result := &RunResult{
		Steps: make([]*StepResult, 0, len(r.workflow.Steps)),
	}

	stepIndex := 0
	gateRetries := make(map[string]int)

	for stepIndex < len(r.workflow.Steps) {
		step := r.workflow.Steps[stepIndex]

		// Call start callback if set
		if r.startCallback != nil {
			info := StepInfo{
				Name:        step.Name,
				Position:    stepIndex + 1, // 1-indexed
				Total:       len(r.workflow.Steps),
				GateRetries: gateRetries[step.Name],
				MaxRetries:  r.workflow.EffectiveMaxGateRetries(),
			}
			r.startCallback(info)
		}

		// Build the prompt with template substitution
		prompt := r.buildPrompt(step.Prompt)

		// Execute the step
		execResult, err := r.executor.ExecuteStep(ctx, step.Name, prompt)
		if err != nil {
			return result, fmt.Errorf("step %q failed: %w", step.Name, err)
		}

		// Update totals
		result.TotalCost += execResult.CostUSD
		result.TotalTokensIn += execResult.TokensIn
		result.TotalTokensOut += execResult.TokensOut

		// Check gate if this is a gate step
		var gateResult GateResult
		if step.Gate {
			gateResult = CheckGate(execResult.Output)
		}

		// Record step result
		stepResult := &StepResult{
			StepName:   step.Name,
			Output:     execResult.Output,
			CostUSD:    execResult.CostUSD,
			TokensIn:   execResult.TokensIn,
			TokensOut:  execResult.TokensOut,
			GateResult: gateResult,
			RetryCount: gateRetries[step.Name],
		}
		result.Steps = append(result.Steps, stepResult)

		// Call callback if set
		if r.callback != nil {
			info := StepInfo{
				Name:        step.Name,
				Position:    stepIndex + 1, // 1-indexed
				Total:       len(r.workflow.Steps),
				GateRetries: gateRetries[step.Name],
				MaxRetries:  r.workflow.EffectiveMaxGateRetries(),
			}
			if err := r.callback(info, execResult, gateResult); err != nil {
				return result, err
			}
		}

		// Handle gate result
		if step.Gate {
			switch gateResult {
			case GatePassed:
				// Continue to next step
				stepIndex++

			case GateFailed:
				// Increment retry count
				gateRetries[step.Name]++

				// Check retry limit
				if gateRetries[step.Name] >= r.workflow.EffectiveMaxGateRetries() {
					return result, fmt.Errorf("%w: step %q failed %d times", ErrMaxGateRetriesExceeded, step.Name, gateRetries[step.Name])
				}

				// Loop back to on_fail step
				if step.OnFail != "" {
					targetIndex := r.workflow.GetStepIndex(step.OnFail)
					if targetIndex < 0 {
						return result, fmt.Errorf("step %q: on_fail target %q not found", step.Name, step.OnFail)
					}
					stepIndex = targetIndex
				}
				// No on_fail specified, just retry this step
				// Don't increment stepIndex

			case GateNotFound:
				// No gate signal found - treat as failure
				gateRetries[step.Name]++
				if gateRetries[step.Name] >= r.workflow.EffectiveMaxGateRetries() {
					return result, fmt.Errorf("%w: step %q did not output gate signal after %d attempts", ErrMaxGateRetriesExceeded, step.Name, gateRetries[step.Name])
				}
				// Retry the step
			}
		} else {
			// Not a gate step, move to next
			stepIndex++
		}
	}

	result.CompletedAllSteps = true
	return result, nil
}

// buildPrompt substitutes template placeholders in the prompt.
func (r *Runner) buildPrompt(template string) string {
	result := template

	// Handle {{files}} placeholder
	if len(r.filePaths) > 0 {
		var fileList strings.Builder
		for _, path := range r.filePaths {
			fileList.WriteString("- ")
			fileList.WriteString(path)
			fileList.WriteString("\n")
		}
		result = strings.ReplaceAll(result, "{{files}}", strings.TrimSuffix(fileList.String(), "\n"))
	}

	// Handle {{plural}} placeholder
	plural := ""
	if len(r.filePaths) > 1 {
		plural = "s"
	}
	result = strings.ReplaceAll(result, "{{plural}}", plural)

	return result
}
