package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/flashingpumpkin/orbit-cli/internal/output"
)

// ErrNotGitRepository is returned when the working directory is not a git repository.
var ErrNotGitRepository = errors.New("not a git repository")

// CheckGitRepository verifies that the given directory is a git repository.
func CheckGitRepository(dir string) error {
	// Check for .git directory or file (worktrees use a .git file)
	gitPath := filepath.Join(dir, ".git")
	info, err := os.Stat(gitPath)
	if err == nil && (info.IsDir() || !info.IsDir()) {
		return nil
	}

	// Fallback: use git rev-parse
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return ErrNotGitRepository
	}

	return nil
}

// GetCurrentBranch returns the current branch name.
func GetCurrentBranch(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// ExecutionResult contains the result of executing Claude.
type ExecutionResult struct {
	Output    string
	CostUSD   float64
	TokensIn  int
	TokensOut int
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
	BranchName   string
	CostUSD      float64
	TokensIn     int
	TokensOut    int
}

// NewSetup creates a new Setup instance.
func NewSetup(executor Executor) *Setup {
	return &Setup{executor: executor}
}

// SetupOptions configures the setup phase.
type SetupOptions struct {
	WorktreeName string // If set, use this name instead of having Claude generate one
}

// Run executes the setup phase, invoking Claude to create a worktree.
func (s *Setup) Run(ctx context.Context, specContent string) (*SetupResult, error) {
	return s.RunWithOptions(ctx, specContent, SetupOptions{})
}

// RunWithOptions executes the setup phase with options.
func (s *Setup) RunWithOptions(ctx context.Context, specContent string, opts SetupOptions) (*SetupResult, error) {
	prompt := buildSetupPrompt(specContent, opts)
	result, err := s.executor.Execute(ctx, prompt)
	if err != nil {
		return nil, err
	}

	path, err := extractWorktreePath(result.Output)
	if err != nil {
		return nil, err
	}

	branch, err := extractBranchName(result.Output)
	if err != nil {
		return nil, err
	}

	return &SetupResult{
		WorktreePath: path,
		BranchName:   branch,
		CostUSD:      result.CostUSD,
		TokensIn:     result.TokensIn,
		TokensOut:    result.TokensOut,
	}, nil
}

// extractWorktreePath parses the WORKTREE_PATH marker from Claude's output.
func extractWorktreePath(output string) (string, error) {
	return extractMarker(output, "WORKTREE_PATH: ", "worktree path")
}

// extractBranchName parses the BRANCH_NAME marker from Claude's output.
func extractBranchName(output string) (string, error) {
	return extractMarker(output, "BRANCH_NAME: ", "branch name")
}

// extractMarker extracts a value from a marker line in the output.
// It first parses stream-json to get actual text content, then searches for markers.
func extractMarker(rawOutput, marker, description string) (string, error) {
	// Parse stream-json to get actual text content
	text := output.ExtractText(rawOutput)

	idx := strings.Index(text, marker)
	if idx == -1 {
		return "", fmt.Errorf("%s not found in output", description)
	}

	// Extract value from after the marker to end of line
	start := idx + len(marker)
	rest := text[start:]

	// Find end of line or end of string
	end := strings.Index(rest, "\n")
	if end == -1 {
		end = len(rest)
	}

	return strings.TrimSpace(rest[:end]), nil
}

// buildSetupPrompt creates the prompt for the setup phase.
func buildSetupPrompt(specContent string, opts SetupOptions) string {
	var nameInstruction string
	if opts.WorktreeName != "" {
		nameInstruction = fmt.Sprintf("Use the name: %s", opts.WorktreeName)
	} else {
		nameInstruction = `Choose a descriptive kebab-case name based on the task (e.g., "add-user-auth", "fix-payment-bug").
If a worktree with your chosen name already exists, append a numeric suffix (e.g., "add-user-auth-2").`
	}

	return fmt.Sprintf(`You are setting up an isolated worktree for development work.

## Task Specification

%s

## Instructions

1. Read and understand the task specification above.
2. %s
3. Create the worktree using: git worktree add -b orbit/<name> .orbit/worktrees/<name> HEAD
4. Output the results in this exact format:

WORKTREE_PATH: .orbit/worktrees/<name>
BRANCH_NAME: orbit/<name>

## Important

- The worktree directory MUST be under .orbit/worktrees/
- The branch name MUST start with orbit/
- Output WORKTREE_PATH: and BRANCH_NAME: markers exactly as shown
- If the branch already exists, pick a different name
`, specContent, nameInstruction)
}
