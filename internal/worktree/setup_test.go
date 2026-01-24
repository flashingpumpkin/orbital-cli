package worktree

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// MockExecutor is a test double that records Execute calls and returns configured results.
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

func TestSetupPhase(t *testing.T) {
	t.Run("invokes executor with spec content", func(t *testing.T) {
		mockExec := &MockExecutor{
			ExecuteResult: &ExecutionResult{
				Output:     "WORKTREE_PATH: .orbit/worktrees/add-user-auth",
				CostUSD:    0.01,
				TokensUsed: 100,
			},
		}

		specContent := "# User Story: Add User Authentication\n\nImplement login functionality."

		setup := NewSetup(mockExec)
		result, err := setup.Run(context.Background(), specContent)

		if err != nil {
			t.Fatalf("Run() error = %v; want nil", err)
		}

		if len(mockExec.ExecuteCalls) != 1 {
			t.Fatalf("Execute() called %d times; want 1", len(mockExec.ExecuteCalls))
		}

		prompt := mockExec.ExecuteCalls[0].Prompt
		if !strings.Contains(prompt, specContent) {
			t.Errorf("prompt does not contain spec content")
		}

		if result.WorktreePath != ".orbit/worktrees/add-user-auth" {
			t.Errorf("WorktreePath = %q; want %q", result.WorktreePath, ".orbit/worktrees/add-user-auth")
		}
	})

	t.Run("prompt instructs Claude to create worktree", func(t *testing.T) {
		mockExec := &MockExecutor{
			ExecuteResult: &ExecutionResult{
				Output:     "WORKTREE_PATH: .orbit/worktrees/test-feature",
				CostUSD:    0.01,
				TokensUsed: 100,
			},
		}

		specContent := "# Test Feature\n\nImplement something."

		setup := NewSetup(mockExec)
		_, err := setup.Run(context.Background(), specContent)
		if err != nil {
			t.Fatalf("Run() error = %v; want nil", err)
		}

		prompt := mockExec.ExecuteCalls[0].Prompt

		// Verify prompt contains instructions for Claude
		requiredInstructions := []string{
			"kebab-case",
			".orbit/worktrees/",
			"orbit/",
			"git worktree",
			"WORKTREE_PATH:",
		}

		for _, instruction := range requiredInstructions {
			if !strings.Contains(prompt, instruction) {
				t.Errorf("prompt missing required instruction: %q", instruction)
			}
		}
	})

	t.Run("returns error on execution failure", func(t *testing.T) {
		mockExec := &MockExecutor{
			ExecuteError: errors.New("execution failed"),
		}

		setup := NewSetup(mockExec)
		_, err := setup.Run(context.Background(), "spec content")

		if err == nil {
			t.Fatal("Run() error = nil; want error")
		}
	})

	t.Run("returns error when path marker not found", func(t *testing.T) {
		mockExec := &MockExecutor{
			ExecuteResult: &ExecutionResult{
				Output: "I created a worktree but forgot to output the path marker.",
			},
		}

		setup := NewSetup(mockExec)
		_, err := setup.Run(context.Background(), "spec content")

		if err == nil {
			t.Fatal("Run() error = nil; want error for missing path")
		}
	})
}
