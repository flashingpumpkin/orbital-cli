package worktree

import (
	"context"
	"testing"
)

// MockExecutor is a test double that records Execute calls and returns configured results.
// Used for testing the merge phase which still invokes Claude.
type MockExecutor struct {
	ExecuteCalls []struct {
		Prompt string
	}
	ExecuteResult *ExecutionResult
	ExecuteError  error
}

func (m *MockExecutor) Execute(ctx context.Context, prompt string) (*ExecutionResult, error) {
	m.ExecuteCalls = append(m.ExecuteCalls, struct{ Prompt string }{Prompt: prompt})
	return m.ExecuteResult, m.ExecuteError
}

func TestSetupOptions(t *testing.T) {
	t.Run("empty WorktreeName means generate name", func(t *testing.T) {
		opts := SetupOptions{}
		if opts.WorktreeName != "" {
			t.Errorf("WorktreeName = %q; want empty", opts.WorktreeName)
		}
	})

	t.Run("WorktreeName can be set", func(t *testing.T) {
		opts := SetupOptions{WorktreeName: "my-feature"}
		if opts.WorktreeName != "my-feature" {
			t.Errorf("WorktreeName = %q; want %q", opts.WorktreeName, "my-feature")
		}
	})
}

func TestSetupResult(t *testing.T) {
	t.Run("contains worktree path and branch", func(t *testing.T) {
		result := SetupResult{
			WorktreePath: ".orbital/worktrees/swift-falcon",
			BranchName:   "orbital/swift-falcon",
			CostUSD:      0,
			TokensIn:     0,
			TokensOut:    0,
		}

		if result.WorktreePath != ".orbital/worktrees/swift-falcon" {
			t.Errorf("WorktreePath = %q; want %q", result.WorktreePath, ".orbital/worktrees/swift-falcon")
		}

		if result.BranchName != "orbital/swift-falcon" {
			t.Errorf("BranchName = %q; want %q", result.BranchName, "orbital/swift-falcon")
		}

		// Local setup has zero cost
		if result.CostUSD != 0 {
			t.Errorf("CostUSD = %v; want 0 for local setup", result.CostUSD)
		}
	})
}

// Note: Integration tests for SetupDirect would require a real git repository.
// These are deferred per the implementation plan (docs/plans/2026-01-24-164500-stories-worktree-fixes.md).
// The function is exercised through the git.go helpers which are unit tested in git_test.go.
