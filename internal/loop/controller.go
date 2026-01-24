// Package loop provides the main execution loop controller for orbit.
package loop

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/flashingpumpkin/orbital/internal/completion"
	"github.com/flashingpumpkin/orbital/internal/config"
	"github.com/flashingpumpkin/orbital/internal/executor"
	"github.com/flashingpumpkin/orbital/internal/spec"
)

// ErrBudgetExceeded is returned when the execution cost exceeds the configured maximum budget.
var ErrBudgetExceeded = errors.New("budget exceeded")

// ErrMaxIterationsReached is returned when the maximum number of iterations is reached without completion.
var ErrMaxIterationsReached = errors.New("max iterations reached")

// LoopState represents the current state of the execution loop.
type LoopState struct {
	// Iteration is the current iteration number (1-indexed).
	Iteration int

	// TotalCost is the cumulative cost in USD across all iterations.
	TotalCost float64

	// TotalTokensIn is the cumulative number of input tokens used across all iterations.
	TotalTokensIn int

	// TotalTokensOut is the cumulative number of output tokens used across all iterations.
	TotalTokensOut int

	// TotalTokens is the cumulative total tokens (in + out) for backward compatibility.
	TotalTokens int

	// StartTime is when the loop execution began.
	StartTime time.Time

	// LastOutput is the output from the most recent iteration.
	LastOutput string

	// Completed indicates whether the task completed successfully (promise detected).
	Completed bool

	// Error contains any error that caused the loop to terminate.
	Error error
}

// ExecutorInterface defines the interface for executing prompts.
// This allows for mocking in tests.
type ExecutorInterface interface {
	Execute(ctx context.Context, prompt string) (*executor.ExecutionResult, error)
}

// IterationCallback is called after each iteration with the current state.
// This allows external code to update persistent state during the loop.
// Parameters: iteration, totalCost, totalTokensIn, totalTokensOut
type IterationCallback func(iteration int, totalCost float64, totalTokensIn, totalTokensOut int) error

// IterationStartCallback is called before each iteration starts.
// Parameters: iteration, maxIterations
type IterationStartCallback func(iteration, maxIterations int)

// StateManager defines an interface for managing queue state.
// This allows the loop to check for queued files after completion.
type StateManager interface {
	// CheckQueue returns any queued files without removing them from the queue.
	CheckQueue() ([]string, error)
	// PopQueue returns and removes all queued files from the queue.
	PopQueue() ([]string, error)
	// MergeFiles adds the given files to the active file list.
	MergeFiles(files []string) error
	// RebuildPrompt rebuilds the prompt with the current active files.
	RebuildPrompt() (string, error)
}

// Verifier defines the interface for verification execution.
// This allows for mocking in tests.
type Verifier interface {
	Verify(ctx context.Context, files []string) (*VerificationResult, error)
}

// Controller manages the execution loop for orbit.
type Controller struct {
	config                 *config.Config
	executor               ExecutorInterface
	detector               *completion.Detector
	iterationCallback      IterationCallback
	iterationStartCallback IterationStartCallback
	stateManager           StateManager
	specFiles              []string
	verifier               Verifier
}

// New creates a new Controller with the given configuration, executor, and detector.
func New(cfg *config.Config, exec ExecutorInterface, det *completion.Detector) *Controller {
	return &Controller{
		config:   cfg,
		executor: exec,
		detector: det,
	}
}

// SetIterationCallback sets a callback function to be called after each iteration.
func (c *Controller) SetIterationCallback(cb IterationCallback) {
	c.iterationCallback = cb
}

// SetIterationStartCallback sets a callback function to be called before each iteration.
func (c *Controller) SetIterationStartCallback(cb IterationStartCallback) {
	c.iterationStartCallback = cb
}

// SetStateManager sets the state manager for queue checking.
func (c *Controller) SetStateManager(sm StateManager) {
	c.stateManager = sm
}

// SetSpecFiles sets the spec file paths for verification.
func (c *Controller) SetSpecFiles(files []string) {
	c.specFiles = files
}

// SetVerifier sets a custom verifier for testing purposes.
func (c *Controller) SetVerifier(v Verifier) {
	c.verifier = v
}

// VerificationResult contains the result of a verification check.
type VerificationResult struct {
	Verified  bool
	Unchecked int
	Checked   int
	Cost      float64
	Tokens    int
}

// verifyCompletion runs a verification check using the checker model (haiku).
// If a custom verifier is set (via SetVerifier), it uses that.
// Otherwise, it creates a fresh executor instance and runs the verification prompt.
// Returns a VerificationResult and any error encountered.
func (c *Controller) verifyCompletion(ctx context.Context) (*VerificationResult, error) {
	// Use custom verifier if set (for testing)
	if c.verifier != nil {
		return c.verifier.Verify(ctx, c.specFiles)
	}

	if len(c.specFiles) == 0 {
		return nil, errors.New("no spec files configured for verification")
	}

	// Create a minimal config for the verification executor
	verifyConfig := &config.Config{
		Model:     c.config.CheckerModel,
		MaxBudget: c.config.MaxBudget,
		// No session ID - fresh session each time
		// No system prompt - just the verification prompt
	}

	// Create a new executor for verification
	verifyExec := executor.New(verifyConfig)

	// Build the verification prompt
	prompt := spec.BuildVerificationPrompt(c.specFiles)

	// Execute verification
	result, err := verifyExec.Execute(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("verification execution failed: %w", err)
	}

	// Parse the response
	verified, unchecked, checked := ParseVerificationResponse(result.Output)

	return &VerificationResult{
		Verified:  verified,
		Unchecked: unchecked,
		Checked:   checked,
		Cost:      result.CostUSD,
		Tokens:    result.TokensIn + result.TokensOut,
	}, nil
}

// ParseVerificationResponse parses the verification output for VERIFIED or INCOMPLETE.
// Returns (verified, unchecked, checked).
func ParseVerificationResponse(output string) (bool, int, int) {
	// Look for VERIFIED: 0 unchecked, N checked
	verifiedRe := regexp.MustCompile(`VERIFIED:\s*0\s*unchecked,\s*(\d+)\s*checked`)
	if matches := verifiedRe.FindStringSubmatch(output); len(matches) > 1 {
		var checked int
		fmt.Sscanf(matches[1], "%d", &checked)
		return true, 0, checked
	}

	// Look for INCOMPLETE: N unchecked, M checked
	incompleteRe := regexp.MustCompile(`INCOMPLETE:\s*(\d+)\s*unchecked,\s*(\d+)\s*checked`)
	if matches := incompleteRe.FindStringSubmatch(output); len(matches) > 2 {
		var unchecked, checked int
		fmt.Sscanf(matches[1], "%d", &unchecked)
		fmt.Sscanf(matches[2], "%d", &checked)
		return false, unchecked, checked
	}

	// If we can't parse the response, assume not verified
	return false, -1, -1
}

// Run executes the main loop, iterating until completion, budget exhaustion,
// or maximum iterations reached.
func (c *Controller) Run(ctx context.Context, prompt string) (*LoopState, error) {
	state := &LoopState{
		StartTime: time.Now(),
	}

	currentPrompt := prompt

	for i := 1; i <= c.config.MaxIterations; i++ {
		state.Iteration = i

		// Check context cancellation before each iteration
		if ctx.Err() != nil {
			state.Error = ctx.Err()
			return state, ctx.Err()
		}

		// Call iteration start callback if set
		if c.iterationStartCallback != nil {
			c.iterationStartCallback(i, c.config.MaxIterations)
		}

		// Execute the prompt
		result, err := c.executor.Execute(ctx, currentPrompt)

		// Update cumulative state from result even if there was an error
		// (e.g., context cancellation still produces partial stats)
		if result != nil {
			state.TotalCost += result.CostUSD
			state.TotalTokensIn += result.TokensIn
			state.TotalTokensOut += result.TokensOut
			state.TotalTokens = state.TotalTokensIn + state.TotalTokensOut
			state.LastOutput = result.Output
		}

		if err != nil {
			state.Error = err
			return state, err
		}

		// Call iteration callback if set
		if c.iterationCallback != nil {
			if err := c.iterationCallback(state.Iteration, state.TotalCost, state.TotalTokensIn, state.TotalTokensOut); err != nil {
				state.Error = err
				return state, err
			}
		}

		// Check for budget exceeded
		if state.TotalCost >= c.config.MaxBudget {
			state.Error = ErrBudgetExceeded
			return state, ErrBudgetExceeded
		}

		// Check for completion
		if c.detector.Check(result.Output) {
			fmt.Println("\nCompletion promise detected. Verifying...")

			// Run verification step
			fmt.Println("Verification: checking spec file(s)...")
			verifyResult, verifyErr := c.verifyCompletion(ctx)

			// Add verification cost to totals
			if verifyResult != nil {
				state.TotalCost += verifyResult.Cost
				state.TotalTokens += verifyResult.Tokens
			}

			// Handle verification errors - continue loop
			if verifyErr != nil {
				fmt.Printf("Verification error: %v. Continuing loop.\n\n", verifyErr)
				continue
			}

			// Handle incomplete verification - continue loop
			if !verifyResult.Verified {
				if verifyResult.Unchecked >= 0 {
					fmt.Printf("Verification: %d unchecked item(s) remain. Continuing loop.\n\n", verifyResult.Unchecked)
				} else {
					fmt.Println("Verification: could not parse response. Continuing loop.")
				}
				continue
			}

			// Verification passed
			fmt.Printf("Verification: all items complete (%d checked).\n", verifyResult.Checked)

			// Check queue for new files if StateManager is set
			if c.stateManager != nil {
				queuedFiles, err := c.stateManager.PopQueue()
				if err != nil {
					state.Error = err
					return state, err
				}

				if len(queuedFiles) > 0 {
					fmt.Printf("Found %d queued file(s), continuing...\n", len(queuedFiles))
					for _, f := range queuedFiles {
						fmt.Printf("  + %s\n", f)
					}
					fmt.Println()

					// Merge queued files into active list
					if err := c.stateManager.MergeFiles(queuedFiles); err != nil {
						state.Error = err
						return state, err
					}

					// Rebuild prompt with new files
					newPrompt, err := c.stateManager.RebuildPrompt()
					if err != nil {
						state.Error = err
						return state, err
					}
					currentPrompt = newPrompt

					// Continue to next iteration with new prompt
					continue
				}
			}

			// No queued files or no state manager - we're done
			fmt.Println("No queued files. Work complete.")
			state.Completed = true
			return state, nil
		}
	}

	// Max iterations reached without completion
	state.Error = ErrMaxIterationsReached
	return state, ErrMaxIterationsReached
}
