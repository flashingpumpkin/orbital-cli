package worktree

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/flashingpumpkin/orbital/internal/output"
)

// mergeSuccessPattern matches various formats of the merge success marker.
// Handles: "MERGE_SUCCESS: true", "MERGE_SUCCESS:true", "merge_success: True", etc.
var mergeSuccessPattern = regexp.MustCompile(`(?i)merge[_\s]*success\s*:\s*(true|false)`)

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
// Uses case-insensitive matching to handle variations like "MERGE_SUCCESS: True",
// "merge_success: true", "MERGE_SUCCESS:true", etc.
func containsSuccessMarker(rawOutput string) bool {
	text := output.ExtractText(rawOutput)
	matches := mergeSuccessPattern.FindStringSubmatch(text)
	if matches == nil {
		return false
	}
	// matches[1] contains the captured "true" or "false"
	return strings.EqualFold(matches[1], "true")
}

// buildMergePrompt creates the prompt for the merge phase.
// The prompt no longer includes directory navigation instructions since the executor
// sets the working directory correctly via cmd.Dir.
func buildMergePrompt(opts MergeOptions) string {
	return fmt.Sprintf(`You are merging changes from a worktree branch back to the original branch.

## Configuration

- Worktree branch: %s
- Original branch: %s

## Instructions

1. Rebase the worktree branch onto the original branch:
   git rebase %s
2. If there are conflicts:
   - Resolve them appropriately
   - Continue the rebase: git rebase --continue
3. Checkout the original branch: git checkout %s
4. Fast-forward merge: git merge --ff-only %s
5. Output the result:

MERGE_SUCCESS: true

## Important

- Only use fast-forward merge (--ff-only) to avoid merge commits
- If conflicts cannot be resolved, output: MERGE_SUCCESS: false
- The rebase should apply commits cleanly when possible
`, opts.BranchName, opts.OriginalBranch, opts.OriginalBranch, opts.OriginalBranch, opts.BranchName)
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
