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
		if !strings.Contains(prompt, opts.WorktreePath) {
			t.Errorf("prompt does not contain worktree path")
		}
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
	if !strings.Contains(prompt, opts.WorktreePath) {
		t.Error("prompt missing worktree path")
	}
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
}
