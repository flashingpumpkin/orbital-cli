package worktree

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/flashingpumpkin/orbit-cli/internal/output"
)

// Merge handles the worktree merge phase.
type Merge struct {
	executor Executor
}

// MergeOptions configures the merge phase.
type MergeOptions struct {
	WorktreePath   string
	BranchName     string
	OriginalBranch string
}

// MergeResult contains the result of the merge phase.
type MergeResult struct {
	Success   bool
	CostUSD   float64
	TokensIn  int
	TokensOut int
}

// NewMerge creates a new Merge instance.
func NewMerge(executor Executor) *Merge {
	return &Merge{executor: executor}
}

// Run executes the merge phase, invoking Claude to rebase and merge.
func (m *Merge) Run(ctx context.Context, opts MergeOptions) (*MergeResult, error) {
	prompt := buildMergePrompt(opts)
	result, err := m.executor.Execute(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Check if merge was successful by looking for success marker
	success := containsSuccessMarker(result.Output)

	return &MergeResult{
		Success:   success,
		CostUSD:   result.CostUSD,
		TokensIn:  result.TokensIn,
		TokensOut: result.TokensOut,
	}, nil
}

// containsSuccessMarker checks if the output indicates successful merge.
// It parses stream-json to get actual text content before searching for markers.
func containsSuccessMarker(rawOutput string) bool {
	text := output.ExtractText(rawOutput)
	const marker = "MERGE_SUCCESS: true"
	return strings.Contains(text, marker)
}

// buildMergePrompt creates the prompt for the merge phase.
func buildMergePrompt(opts MergeOptions) string {
	return fmt.Sprintf(`You are merging changes from a worktree branch back to the original branch.

## Configuration

- Worktree path: %s
- Worktree branch: %s
- Original branch: %s

## Instructions

1. Navigate to the worktree directory
2. Rebase the worktree branch onto the original branch:
   git rebase %s
3. If there are conflicts:
   - Resolve them appropriately
   - Continue the rebase: git rebase --continue
4. Navigate to the main repository (parent of .orbit directory)
5. Checkout the original branch: git checkout %s
6. Fast-forward merge: git merge --ff-only %s
7. Output the result:

MERGE_SUCCESS: true

## Important

- Only use fast-forward merge (--ff-only) to avoid merge commits
- If conflicts cannot be resolved, output: MERGE_SUCCESS: false
- The rebase should apply commits cleanly when possible
`, opts.WorktreePath, opts.BranchName, opts.OriginalBranch, opts.OriginalBranch, opts.OriginalBranch, opts.BranchName)
}

// Cleanup handles worktree cleanup after successful merge.
type Cleanup struct {
	workingDir string
}

// NewCleanup creates a new Cleanup instance.
func NewCleanup(workingDir string) *Cleanup {
	return &Cleanup{workingDir: workingDir}
}

// Run removes the worktree and its branch.
func (c *Cleanup) Run(worktreePath, branchName string) error {
	// Remove the worktree
	removeCmd := exec.Command("git", "-C", c.workingDir, "worktree", "remove", worktreePath, "--force")
	removeOutput, err := removeCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove worktree %q: %w\ngit output: %s", worktreePath, err, string(removeOutput))
	}

	// Delete the branch
	branchCmd := exec.Command("git", "-C", c.workingDir, "branch", "-d", branchName)
	branchOutput, err := branchCmd.CombinedOutput()
	if err != nil {
		// Branch deletion might fail if it wasn't fully merged, try force delete
		forceBranchCmd := exec.Command("git", "-C", c.workingDir, "branch", "-D", branchName)
		forceOutput, forceErr := forceBranchCmd.CombinedOutput()
		if forceErr != nil {
			return fmt.Errorf("failed to delete branch %q: %w\ngit branch -d output: %s\ngit branch -D output: %s",
				branchName, forceErr, string(branchOutput), string(forceOutput))
		}
	}

	return nil
}
