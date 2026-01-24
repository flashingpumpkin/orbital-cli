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

// streamJSON creates a stream-json formatted output line for a text delta.
func streamJSON(text string) string {
	// Escape quotes and newlines in text for JSON
	escaped := strings.ReplaceAll(text, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	escaped = strings.ReplaceAll(escaped, "\n", `\n`)
	return `{"type":"content_block_delta","delta":{"text":"` + escaped + `"}}`
}

func TestSetupPhase(t *testing.T) {
	t.Run("invokes executor with spec content", func(t *testing.T) {
		// Use stream-json format for output
		output := streamJSON("WORKTREE_PATH: .orbit/worktrees/add-user-auth\n") + "\n" +
			streamJSON("BRANCH_NAME: orbit/add-user-auth")
		mockExec := &MockExecutor{
			ExecuteResult: &ExecutionResult{
				Output:   output,
				CostUSD:  0.01,
				TokensIn: 50, TokensOut: 50,
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

		if result.BranchName != "orbit/add-user-auth" {
			t.Errorf("BranchName = %q; want %q", result.BranchName, "orbit/add-user-auth")
		}
	})

	t.Run("prompt instructs Claude to create worktree", func(t *testing.T) {
		output := streamJSON("WORKTREE_PATH: .orbit/worktrees/test-feature\n") + "\n" +
			streamJSON("BRANCH_NAME: orbit/test-feature")
		mockExec := &MockExecutor{
			ExecuteResult: &ExecutionResult{
				Output:   output,
				CostUSD:  0.01,
				TokensIn: 50, TokensOut: 50,
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
			"BRANCH_NAME:",
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
		// Use stream-json format but without the path marker
		output := streamJSON("I created a worktree but forgot to output the path marker.")
		mockExec := &MockExecutor{
			ExecuteResult: &ExecutionResult{
				Output: output,
			},
		}

		setup := NewSetup(mockExec)
		_, err := setup.Run(context.Background(), "spec content")

		if err == nil {
			t.Fatal("Run() error = nil; want error for missing path")
		}
	})

	t.Run("returns error when branch marker not found", func(t *testing.T) {
		// Use stream-json format with path but no branch marker
		output := streamJSON("WORKTREE_PATH: .orbit/worktrees/test\n") + "\n" +
			streamJSON("No branch marker here.")
		mockExec := &MockExecutor{
			ExecuteResult: &ExecutionResult{
				Output: output,
			},
		}

		setup := NewSetup(mockExec)
		_, err := setup.Run(context.Background(), "spec content")

		if err == nil {
			t.Fatal("Run() error = nil; want error for missing branch")
		}

		if !strings.Contains(err.Error(), "branch name") {
			t.Errorf("error = %v; want error mentioning branch name", err)
		}
	})

	t.Run("uses provided worktree name when specified", func(t *testing.T) {
		output := streamJSON("WORKTREE_PATH: .orbit/worktrees/my-custom-name\n") + "\n" +
			streamJSON("BRANCH_NAME: orbit/my-custom-name")
		mockExec := &MockExecutor{
			ExecuteResult: &ExecutionResult{
				Output:   output,
				CostUSD:  0.01,
				TokensIn: 50, TokensOut: 50,
			},
		}

		setup := NewSetup(mockExec)
		_, err := setup.RunWithOptions(context.Background(), "spec content", SetupOptions{
			WorktreeName: "my-custom-name",
		})
		if err != nil {
			t.Fatalf("RunWithOptions() error = %v; want nil", err)
		}

		prompt := mockExec.ExecuteCalls[0].Prompt

		// Should contain the custom name instruction
		if !strings.Contains(prompt, "my-custom-name") {
			t.Errorf("prompt does not contain custom name")
		}

		// Should NOT contain the instruction to choose a name
		if strings.Contains(prompt, "Choose a descriptive") {
			t.Errorf("prompt should not ask Claude to choose a name when one is provided")
		}
	})

	t.Run("captures cost and tokens from execution", func(t *testing.T) {
		output := streamJSON("WORKTREE_PATH: .orbit/worktrees/test\n") + "\n" +
			streamJSON("BRANCH_NAME: orbit/test")
		mockExec := &MockExecutor{
			ExecuteResult: &ExecutionResult{
				Output:    output,
				CostUSD:   0.05,
				TokensIn:  200,
				TokensOut: 300,
			},
		}

		setup := NewSetup(mockExec)
		result, err := setup.Run(context.Background(), "spec content")
		if err != nil {
			t.Fatalf("Run() error = %v; want nil", err)
		}

		if result.CostUSD != 0.05 {
			t.Errorf("CostUSD = %v; want 0.05", result.CostUSD)
		}

		if result.TokensIn != 200 {
			t.Errorf("TokensIn = %d; want 200", result.TokensIn)
		}

		if result.TokensOut != 300 {
			t.Errorf("TokensOut = %d; want 300", result.TokensOut)
		}
	})
}

func TestExtractMarker(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		marker      string
		want        string
		wantErr     bool
		errContains string
	}{
		{
			name: "extracts value from middle of output",
			output: streamJSON("Some text\n") + "\n" +
				streamJSON("WORKTREE_PATH: .orbit/worktrees/test\n") + "\n" +
				streamJSON("More text"),
			marker: "WORKTREE_PATH: ",
			want:   ".orbit/worktrees/test",
		},
		{
			name: "extracts value at end of output",
			output: streamJSON("Some text\n") + "\n" +
				streamJSON("WORKTREE_PATH: .orbit/worktrees/test"),
			marker: "WORKTREE_PATH: ",
			want:   ".orbit/worktrees/test",
		},
		{
			name:   "trims whitespace from value",
			output: streamJSON("WORKTREE_PATH:   .orbit/worktrees/test  \n"),
			marker: "WORKTREE_PATH: ",
			want:   ".orbit/worktrees/test",
		},
		{
			name:        "returns error when marker not found",
			output:      streamJSON("No marker here"),
			marker:      "WORKTREE_PATH: ",
			wantErr:     true,
			errContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractMarker(tt.output, tt.marker, "test value")

			if tt.wantErr {
				if err == nil {
					t.Fatal("extractMarker() error = nil; want error")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v; want error containing %q", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("extractMarker() error = %v; want nil", err)
			}

			if got != tt.want {
				t.Errorf("extractMarker() = %q; want %q", got, tt.want)
			}
		})
	}
}
