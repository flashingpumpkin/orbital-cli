package worktree

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// streamJSONMerge creates a stream-json formatted output line for a text delta.
func streamJSONMerge(text string) string {
	escaped := strings.ReplaceAll(text, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	escaped = strings.ReplaceAll(escaped, "\n", `\n`)
	return `{"type":"content_block_delta","delta":{"text":"` + escaped + `"}}`
}

func TestMergePhase(t *testing.T) {
	t.Run("invokes executor with merge prompt", func(t *testing.T) {
		mockExec := &MockExecutor{
			ExecuteResult: &ExecutionResult{
				Output:    streamJSONMerge("MERGE_SUCCESS: true"),
				CostUSD:   0.02,
				TokensIn:  100,
				TokensOut: 100,
			},
		}

		opts := MergeOptions{
			WorktreePath:   ".orbital/worktrees/add-user-auth",
			BranchName:     "orbit/add-user-auth",
			OriginalBranch: "main",
		}

		merge := NewMerge(mockExec)
		result, err := merge.Run(context.Background(), opts)

		if err != nil {
			t.Fatalf("Run() error = %v; want nil", err)
		}

		if len(mockExec.ExecuteCalls) != 1 {
			t.Fatalf("Execute() called %d times; want 1", len(mockExec.ExecuteCalls))
		}

		prompt := mockExec.ExecuteCalls[0].Prompt
		// Worktree path is no longer in the prompt - directory is set via cmd.Dir
		if !strings.Contains(prompt, opts.BranchName) {
			t.Errorf("prompt does not contain branch name")
		}
		if !strings.Contains(prompt, opts.OriginalBranch) {
			t.Errorf("prompt does not contain original branch")
		}

		if !result.Success {
			t.Errorf("Success = false; want true")
		}
		if result.CostUSD != 0.02 {
			t.Errorf("CostUSD = %f; want 0.02", result.CostUSD)
		}
		if result.TokensIn != 100 {
			t.Errorf("TokensIn = %d; want 100", result.TokensIn)
		}
		if result.TokensOut != 100 {
			t.Errorf("TokensOut = %d; want 100", result.TokensOut)
		}
	})

	t.Run("returns false success when marker missing", func(t *testing.T) {
		mockExec := &MockExecutor{
			ExecuteResult: &ExecutionResult{
				Output:   streamJSONMerge("Some output without success marker"),
				CostUSD:  0.01,
				TokensIn: 50, TokensOut: 50,
			},
		}

		merge := NewMerge(mockExec)
		result, err := merge.Run(context.Background(), MergeOptions{
			WorktreePath:   ".orbital/worktrees/test",
			BranchName:     "orbit/test",
			OriginalBranch: "main",
		})

		if err != nil {
			t.Fatalf("Run() error = %v; want nil", err)
		}

		if result.Success {
			t.Errorf("Success = true; want false")
		}
	})

	t.Run("returns false success when marker explicitly false", func(t *testing.T) {
		mockExec := &MockExecutor{
			ExecuteResult: &ExecutionResult{
				Output:   streamJSONMerge("MERGE_SUCCESS: false"),
				CostUSD:  0.01,
				TokensIn: 50, TokensOut: 50,
			},
		}

		merge := NewMerge(mockExec)
		result, err := merge.Run(context.Background(), MergeOptions{
			WorktreePath:   ".orbital/worktrees/test",
			BranchName:     "orbit/test",
			OriginalBranch: "main",
		})

		if err != nil {
			t.Fatalf("Run() error = %v; want nil", err)
		}

		if result.Success {
			t.Errorf("Success = true; want false (marker was 'false')")
		}
	})

	t.Run("returns error when executor fails", func(t *testing.T) {
		expectedErr := errors.New("executor failed")
		mockExec := &MockExecutor{
			ExecuteError: expectedErr,
		}

		merge := NewMerge(mockExec)
		_, err := merge.Run(context.Background(), MergeOptions{
			WorktreePath:   ".orbital/worktrees/test",
			BranchName:     "orbit/test",
			OriginalBranch: "main",
		})

		if err != expectedErr {
			t.Errorf("Run() error = %v; want %v", err, expectedErr)
		}
	})

	t.Run("detects success marker in multi-line output", func(t *testing.T) {
		output := streamJSONMerge("Rebasing branch onto main...\n") + "\n" +
			streamJSONMerge("Successfully rebased.\n") + "\n" +
			streamJSONMerge("MERGE_SUCCESS: true\n") + "\n" +
			streamJSONMerge("Done.")
		mockExec := &MockExecutor{
			ExecuteResult: &ExecutionResult{
				Output:    output,
				CostUSD:   0.03,
				TokensIn:  150,
				TokensOut: 150,
			},
		}

		merge := NewMerge(mockExec)
		result, err := merge.Run(context.Background(), MergeOptions{
			WorktreePath:   ".orbital/worktrees/feature",
			BranchName:     "orbit/feature",
			OriginalBranch: "develop",
		})

		if err != nil {
			t.Fatalf("Run() error = %v; want nil", err)
		}

		if !result.Success {
			t.Errorf("Success = false; want true (marker found in output)")
		}
	})
}

func TestContainsSuccessMarker(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{
			name:   "exact marker",
			output: streamJSONMerge("MERGE_SUCCESS: true"),
			want:   true,
		},
		{
			name: "marker with surrounding text",
			output: streamJSONMerge("Done rebasing.\n") + "\n" +
				streamJSONMerge("MERGE_SUCCESS: true\n") + "\n" +
				streamJSONMerge("Cleanup complete."),
			want: true,
		},
		{
			name: "marker at end",
			output: streamJSONMerge("Merge completed successfully.\n") + "\n" +
				streamJSONMerge("MERGE_SUCCESS: true"),
			want: true,
		},
		{
			name:   "no marker",
			output: streamJSONMerge("Merge failed with conflicts."),
			want:   false,
		},
		{
			name:   "empty output",
			output: "",
			want:   false,
		},
		{
			name:   "false marker",
			output: streamJSONMerge("MERGE_SUCCESS: false"),
			want:   false,
		},
		{
			name:   "partial marker",
			output: streamJSONMerge("MERGE_SUCCESS:"),
			want:   false,
		},
		// Case-insensitive matching tests
		{
			name:   "lowercase marker",
			output: streamJSONMerge("merge_success: true"),
			want:   true,
		},
		{
			name:   "mixed case marker",
			output: streamJSONMerge("Merge_Success: True"),
			want:   true,
		},
		{
			name:   "uppercase TRUE",
			output: streamJSONMerge("MERGE_SUCCESS: TRUE"),
			want:   true,
		},
		{
			name:   "no space after colon",
			output: streamJSONMerge("MERGE_SUCCESS:true"),
			want:   true,
		},
		{
			name:   "extra spaces",
			output: streamJSONMerge("MERGE_SUCCESS:   true"),
			want:   true,
		},
		{
			name:   "underscore vs space - merge success",
			output: streamJSONMerge("MERGE SUCCESS: true"),
			want:   true,
		},
		{
			name:   "lowercase false",
			output: streamJSONMerge("merge_success: false"),
			want:   false,
		},
		{
			name:   "mixed case False",
			output: streamJSONMerge("MERGE_SUCCESS: False"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsSuccessMarker(tt.output)
			if got != tt.want {
				t.Errorf("containsSuccessMarker() = %v; want %v", got, tt.want)
			}
		})
	}
}

func TestBuildMergePrompt(t *testing.T) {
	opts := MergeOptions{
		WorktreePath:   ".orbital/worktrees/my-feature",
		BranchName:     "orbit/my-feature",
		OriginalBranch: "main",
	}

	prompt := buildMergePrompt(opts)

	// Check that prompt contains all required elements
	// Note: WorktreePath is no longer in the prompt - directory is set via cmd.Dir
	if !strings.Contains(prompt, opts.BranchName) {
		t.Error("prompt missing branch name")
	}
	if !strings.Contains(prompt, opts.OriginalBranch) {
		t.Error("prompt missing original branch")
	}
	if !strings.Contains(prompt, "git rebase") {
		t.Error("prompt missing rebase instruction")
	}
	if !strings.Contains(prompt, "--ff-only") {
		t.Error("prompt missing fast-forward instruction")
	}
	if !strings.Contains(prompt, "MERGE_SUCCESS: true") {
		t.Error("prompt missing success marker instruction")
	}

	// Verify that navigation instructions have been removed
	if strings.Contains(prompt, "Navigate to") {
		t.Error("prompt should not contain navigation instructions (working directory is set via cmd.Dir)")
	}
}

func TestCleanup(t *testing.T) {
	t.Run("creates cleanup instance with working directory", func(t *testing.T) {
		cleanup := NewCleanup("/path/to/repo")
		if cleanup == nil {
			t.Fatal("NewCleanup returned nil")
		}
		if cleanup.workingDir != "/path/to/repo" {
			t.Errorf("workingDir = %q; want %q", cleanup.workingDir, "/path/to/repo")
		}
	})

	t.Run("Run accepts context parameter", func(t *testing.T) {
		// This test verifies the method signature accepts context.
		// We use a cancelled context to ensure it fails quickly.
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		cleanup := NewCleanup("/nonexistent/path")
		err := cleanup.Run(ctx, "/nonexistent/worktree", "nonexistent-branch")

		// The command should fail (either due to cancellation or because the path doesn't exist)
		if err == nil {
			t.Error("Run() should return error for cancelled context or nonexistent path")
		}
	})

	t.Run("Run returns error for nonexistent worktree", func(t *testing.T) {
		cleanup := NewCleanup("/nonexistent/path")
		err := cleanup.Run(context.Background(), "/nonexistent/worktree", "nonexistent-branch")

		if err == nil {
			t.Error("Run() should return error for nonexistent worktree")
		}
		if !strings.Contains(err.Error(), "failed to remove worktree") {
			t.Errorf("error should mention worktree removal failure: %v", err)
		}
	})
}
