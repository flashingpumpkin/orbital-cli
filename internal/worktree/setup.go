package worktree

import (
	"context"
	"errors"
	"strings"
)

// ExecutionResult contains the result of executing Claude.
type ExecutionResult struct {
	Output     string
	CostUSD    float64
	TokensUsed int
}

// Executor runs Claude with a prompt and returns the result.
type Executor interface {
	Execute(ctx context.Context, prompt string) (*ExecutionResult, error)
}

// Setup handles the worktree setup phase.
type Setup struct {
	executor Executor
}

// SetupResult contains the result of the setup phase.
type SetupResult struct {
	WorktreePath string
}

// NewSetup creates a new Setup instance.
func NewSetup(executor Executor) *Setup {
	return &Setup{executor: executor}
}

// Run executes the setup phase, invoking Claude to create a worktree.
func (s *Setup) Run(ctx context.Context, specContent string) (*SetupResult, error) {
	result, err := s.executor.Execute(ctx, specContent)
	if err != nil {
		return nil, err
	}

	path, err := extractWorktreePath(result.Output)
	if err != nil {
		return nil, err
	}

	return &SetupResult{WorktreePath: path}, nil
}

// extractWorktreePath parses the WORKTREE_PATH marker from Claude's output.
func extractWorktreePath(output string) (string, error) {
	const marker = "WORKTREE_PATH: "
	idx := strings.Index(output, marker)
	if idx == -1 {
		return "", errors.New("worktree path not found in output")
	}

	// Extract path from after the marker to end of line
	start := idx + len(marker)
	rest := output[start:]

	// Find end of line or end of string
	end := strings.Index(rest, "\n")
	if end == -1 {
		end = len(rest)
	}

	return strings.TrimSpace(rest[:end]), nil
}
