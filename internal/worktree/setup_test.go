package worktree

import (
	"context"
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

func TestSetupPhase_InvokesClaude(t *testing.T) {
	// Setup phase should invoke Claude with a prompt containing the spec content
	// to create a worktree with a descriptive name.

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

	// Verify Claude was invoked
	if len(mockExec.ExecuteCalls) != 1 {
		t.Fatalf("Execute() called %d times; want 1", len(mockExec.ExecuteCalls))
	}

	// Verify the prompt contains the spec content
	prompt := mockExec.ExecuteCalls[0].Prompt
	if !containsString(prompt, specContent) {
		t.Errorf("prompt does not contain spec content")
	}

	// Verify the worktree path was captured from output
	if result.WorktreePath != ".orbit/worktrees/add-user-auth" {
		t.Errorf("WorktreePath = %q; want %q", result.WorktreePath, ".orbit/worktrees/add-user-auth")
	}
}

func TestSetupPhase_ReturnsErrorOnExecutionFailure(t *testing.T) {
	mockExec := &MockExecutor{
		ExecuteError: context.DeadlineExceeded,
	}

	setup := NewSetup(mockExec)
	_, err := setup.Run(context.Background(), "spec content")

	if err == nil {
		t.Fatal("Run() error = nil; want error")
	}
}

func TestSetupPhase_ReturnsErrorWhenPathNotFound(t *testing.T) {
	// If Claude's output doesn't contain the expected WORKTREE_PATH marker,
	// setup should return an error.

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
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
