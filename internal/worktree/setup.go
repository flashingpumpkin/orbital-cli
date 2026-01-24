package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
// This is used by the merge phase which still invokes Claude.
type ExecutionResult struct {
	Output    string
	CostUSD   float64
	TokensIn  int
	TokensOut int
}

// Executor runs Claude with a prompt and returns the result.
// This is used by the merge phase which still invokes Claude.
type Executor interface {
	Execute(ctx context.Context, prompt string) (*ExecutionResult, error)
}

// SetupResult contains the result of the setup phase.
type SetupResult struct {
	WorktreePath string
	BranchName   string
	CostUSD      float64
	TokensIn     int
	TokensOut    int
}

// SetupOptions configures the setup phase.
type SetupOptions struct {
	WorktreeName string // If set, use this name instead of generating one
}

// SetupDirect creates a worktree directly using local name generation.
// This does not invoke Claude - it generates a name locally and runs git commands directly.
func SetupDirect(workingDir string, opts SetupOptions) (*SetupResult, error) {
	// Determine the worktree name
	var name string
	if opts.WorktreeName != "" {
		// Use the user-provided name
		name = opts.WorktreeName
	} else {
		// Generate a unique name locally
		existingNames, err := ListWorktreeNames(workingDir)
		if err != nil {
			// Non-fatal - just use an empty exclusion list
			existingNames = nil
		}
		name = GenerateUniqueName(existingNames)
	}

	// Create the worktree using direct git commands
	if err := CreateWorktree(workingDir, name); err != nil {
		return nil, fmt.Errorf("failed to create worktree: %w", err)
	}

	// Convert worktree path to absolute to ensure it works regardless of
	// how spec file paths are provided (relative or absolute)
	absWorktreePath, err := filepath.Abs(WorktreePath(name))
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute worktree path: %w", err)
	}

	return &SetupResult{
		WorktreePath: absWorktreePath,
		BranchName:   BranchName(name),
		// No cost since we don't invoke Claude
		CostUSD:   0,
		TokensIn:  0,
		TokensOut: 0,
	}, nil
}
