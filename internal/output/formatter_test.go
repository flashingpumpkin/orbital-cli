package output

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	orberrors "github.com/flashingpumpkin/orbital/internal/errors"
)

func TestNewFormatter(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)
	if f == nil {
		t.Fatal("NewFormatter() returned nil")
	}
}

func TestNewFormatter_Fields(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(true, false, &buf)

	if !f.verbose {
		t.Error("expected verbose to be true")
	}
	if f.quiet {
		t.Error("expected quiet to be false")
	}
	if f.writer != &buf {
		t.Error("expected writer to be set")
	}
}

func TestNewFormatter_QuietMode(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, true, &buf)

	if f.verbose {
		t.Error("expected verbose to be false")
	}
	if !f.quiet {
		t.Error("expected quiet to be true")
	}
}

func TestNewFormatter_NoColorEnvVar(t *testing.T) {
	// Set NO_COLOR environment variable
	if err := os.Setenv("NO_COLOR", "1"); err != nil {
		t.Fatalf("failed to set NO_COLOR env var: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("NO_COLOR"); err != nil {
			t.Errorf("failed to unset NO_COLOR env var: %v", err)
		}
	}()

	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	if !f.noColor {
		t.Error("expected noColor to be true when NO_COLOR env var is set")
	}
}

func TestNewFormatter_NoColorEnvVarEmpty(t *testing.T) {
	// Ensure NO_COLOR is not set
	if err := os.Unsetenv("NO_COLOR"); err != nil {
		t.Fatalf("failed to unset NO_COLOR env var: %v", err)
	}

	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	if f.noColor {
		t.Error("expected noColor to be false when NO_COLOR env var is not set")
	}
}

func TestPrintRichBanner(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	cfg := BannerConfig{
		SpecFile:      "docs/plans/story.md",
		ContextFiles:  []string{"context1.md", "context2.md"},
		WorkflowName:  "spec-driven",
		WorkflowSteps: 3,
		HasGates:      true,
		Model:         "opus",
		CheckerModel:  "haiku",
		MaxIterations: 50,
		Budget:        100.00,
		Timeout:       30 * time.Minute,
		WorkingDir:    "/home/user/project",
		NotesFile:     "docs/notes/notes.md",
	}

	f.PrintRichBanner(cfg)
	output := buf.String()

	// Check for key elements
	if !strings.Contains(output, "Orbit") {
		t.Error("expected output to contain 'Orbit'")
	}
	if !strings.Contains(output, "docs/plans/story.md") {
		t.Error("expected output to contain spec file path")
	}
	if !strings.Contains(output, "2 file(s)") {
		t.Error("expected output to contain context file count")
	}
	if !strings.Contains(output, "context1.md") {
		t.Error("expected output to contain first context file")
	}
	if !strings.Contains(output, "spec-driven") {
		t.Error("expected output to contain workflow name")
	}
	if !strings.Contains(output, "3 steps, with gates") {
		t.Error("expected output to contain workflow steps with gates")
	}
	if !strings.Contains(output, "opus") {
		t.Error("expected output to contain model")
	}
	if !strings.Contains(output, "haiku") {
		t.Error("expected output to contain checker model")
	}
	if !strings.Contains(output, "50") {
		t.Error("expected output to contain max iterations")
	}
	if !strings.Contains(output, "100.00") {
		t.Error("expected output to contain budget")
	}
	if !strings.Contains(output, "30m") {
		t.Error("expected output to contain timeout")
	}
	if !strings.Contains(output, "/home/user/project") {
		t.Error("expected output to contain working dir")
	}
	if !strings.Contains(output, "docs/notes/notes.md") {
		t.Error("expected output to contain notes file")
	}
}

func TestPrintRichBanner_WithSessionID(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	cfg := BannerConfig{
		SpecFile:      "spec.md",
		WorkflowName:  "fast",
		WorkflowSteps: 1,
		Model:         "sonnet",
		CheckerModel:  "haiku",
		MaxIterations: 10,
		Budget:        10.00,
		Timeout:       5 * time.Minute,
		WorkingDir:    ".",
		NotesFile:     "notes.md",
		SessionID:     "abc123",
	}

	f.PrintRichBanner(cfg)
	output := buf.String()

	if !strings.Contains(output, "abc123") {
		t.Error("expected output to contain session ID")
	}
	if !strings.Contains(output, "Resuming") {
		t.Error("expected output to indicate resuming session")
	}
}

func TestPrintRichBanner_DryRunAndDebug(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	cfg := BannerConfig{
		SpecFile:      "spec.md",
		WorkflowName:  "fast",
		WorkflowSteps: 1,
		Model:         "sonnet",
		CheckerModel:  "haiku",
		MaxIterations: 10,
		Budget:        10.00,
		Timeout:       5 * time.Minute,
		WorkingDir:    ".",
		NotesFile:     "notes.md",
		DryRun:        true,
		Debug:         true,
	}

	f.PrintRichBanner(cfg)
	output := buf.String()

	if !strings.Contains(output, "DRY RUN") {
		t.Error("expected output to contain DRY RUN")
	}
	if !strings.Contains(output, "Debug") || !strings.Contains(output, "enabled") {
		t.Error("expected output to indicate debug mode")
	}
}

func TestPrintRichBanner_QuietMode(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, true, &buf)

	cfg := BannerConfig{
		SpecFile:      "spec.md",
		WorkflowName:  "fast",
		WorkflowSteps: 1,
		Model:         "sonnet",
		CheckerModel:  "haiku",
		MaxIterations: 10,
		Budget:        10.00,
		Timeout:       5 * time.Minute,
		WorkingDir:    ".",
		NotesFile:     "notes.md",
	}

	f.PrintRichBanner(cfg)
	output := buf.String()

	if output != "" {
		t.Errorf("expected no output in quiet mode, got: %q", output)
	}
}

func TestPrintRichBanner_NoGates(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	cfg := BannerConfig{
		SpecFile:      "spec.md",
		WorkflowName:  "fast",
		WorkflowSteps: 1,
		HasGates:      false,
		Model:         "sonnet",
		CheckerModel:  "haiku",
		MaxIterations: 10,
		Budget:        10.00,
		Timeout:       5 * time.Minute,
		WorkingDir:    ".",
		NotesFile:     "notes.md",
	}

	f.PrintRichBanner(cfg)
	output := buf.String()

	if !strings.Contains(output, "1 step)") {
		t.Errorf("expected output to contain '1 step)', got: %q", output)
	}
}

func TestPrintLoopSummary_Completed(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	summary := LoopSummary{
		Iterations:  5,
		TotalCost:   0.25,
		TotalTokens: 7500,
		Duration:    2*time.Minute + 30*time.Second,
		Completed:   true,
	}

	f.PrintLoopSummary(summary)
	output := buf.String()

	if !strings.Contains(output, "Summary") {
		t.Error("expected output to contain 'Summary'")
	}
	if !strings.Contains(output, "5") {
		t.Error("expected output to contain iteration count")
	}
	if !strings.Contains(output, "0.25") {
		t.Error("expected output to contain cost")
	}
	if !strings.Contains(output, "7500") {
		t.Error("expected output to contain token count")
	}
	if !strings.Contains(output, "2m") {
		t.Error("expected output to contain duration")
	}
	if !strings.Contains(output, "COMPLETED") {
		t.Error("expected output to indicate completion")
	}
}

func TestPrintLoopSummary_Error(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	summary := LoopSummary{
		Iterations:  10,
		TotalCost:   0.50,
		TotalTokens: 15000,
		Duration:    5 * time.Minute,
		Completed:   false,
		Error:       orberrors.ErrMaxIterationsReached,
	}

	f.PrintLoopSummary(summary)
	output := buf.String()

	// The implementation shows "MAX ITERATIONS REACHED" for this error type
	if !strings.Contains(output, "MAX ITERATIONS REACHED") {
		t.Errorf("expected output to indicate max iterations reached, got: %s", output)
	}
}

func TestPrintLoopSummary_NotCompleted(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	summary := LoopSummary{
		Iterations:  3,
		TotalCost:   0.15,
		TotalTokens: 4500,
		Duration:    1 * time.Minute,
		Completed:   false,
	}

	f.PrintLoopSummary(summary)
	output := buf.String()

	if !strings.Contains(output, "NOT COMPLETED") {
		t.Error("expected output to indicate not completed")
	}
}

func TestPrintLoopSummary_TokenBreakdown(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	summary := LoopSummary{
		Iterations:  5,
		TotalCost:   0.25,
		TotalTokens: 10000, // Should be ignored when TokensIn/Out are set
		TokensIn:    7500,
		TokensOut:   2500,
		Duration:    2 * time.Minute,
		Completed:   true,
	}

	f.PrintLoopSummary(summary)
	output := buf.String()

	// Should show detailed token breakdown
	if !strings.Contains(output, "7500 in") {
		t.Errorf("expected output to show tokens in, got: %s", output)
	}
	if !strings.Contains(output, "2500 out") {
		t.Errorf("expected output to show tokens out, got: %s", output)
	}
}

func TestPrintLoopSummary_Interrupted(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	summary := LoopSummary{
		Iterations: 3,
		TotalCost:  0.15,
		TokensIn:   5000,
		TokensOut:  1500,
		Duration:   1 * time.Minute,
		Completed:  false,
		Error:      context.Canceled,
		SessionID:  "abc123def",
	}

	f.PrintLoopSummary(summary)
	output := buf.String()

	if !strings.Contains(output, "INTERRUPTED") {
		t.Errorf("expected output to show INTERRUPTED status, got: %s", output)
	}
	if !strings.Contains(output, "Resume with") {
		t.Errorf("expected output to show resume instructions, got: %s", output)
	}
	if !strings.Contains(output, "orbital continue") {
		t.Errorf("expected output to show orbital continue command, got: %s", output)
	}
}

func TestPrintLoopSummary_BudgetExceeded(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	summary := LoopSummary{
		Iterations: 10,
		TotalCost:  5.00,
		Duration:   10 * time.Minute,
		Completed:  false,
		Error:      orberrors.ErrBudgetExceeded,
		SessionID:  "abc123def",
	}

	f.PrintLoopSummary(summary)
	output := buf.String()

	if !strings.Contains(output, "BUDGET EXCEEDED") {
		t.Errorf("expected output to show BUDGET EXCEEDED status, got: %s", output)
	}
	// Should also show resume instructions since session can be resumed
	if !strings.Contains(output, "Resume with") {
		t.Errorf("expected output to show resume instructions, got: %s", output)
	}
}

func TestPrintLoopSummary_Timeout(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	summary := LoopSummary{
		Iterations: 1,
		TotalCost:  0.05,
		Duration:   5 * time.Minute,
		Completed:  false,
		Error:      context.DeadlineExceeded,
		SessionID:  "abc123def",
	}

	f.PrintLoopSummary(summary)
	output := buf.String()

	if !strings.Contains(output, "TIMEOUT") {
		t.Errorf("expected output to show TIMEOUT status, got: %s", output)
	}
	// Should also show resume instructions since session can be resumed
	if !strings.Contains(output, "Resume with") {
		t.Errorf("expected output to show resume instructions, got: %s", output)
	}
}

func TestPrintLoopSummary_WrappedErrors(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus string
	}{
		{
			name:           "wrapped context canceled",
			err:            fmt.Errorf("execution failed: %w", context.Canceled),
			expectedStatus: "INTERRUPTED",
		},
		{
			name:           "wrapped max iterations",
			err:            fmt.Errorf("loop terminated: %w", orberrors.ErrMaxIterationsReached),
			expectedStatus: "MAX ITERATIONS REACHED",
		},
		{
			name:           "wrapped budget exceeded",
			err:            fmt.Errorf("cost limit hit: %w", orberrors.ErrBudgetExceeded),
			expectedStatus: "BUDGET EXCEEDED",
		},
		{
			name:           "wrapped deadline exceeded",
			err:            fmt.Errorf("iteration timed out: %w", context.DeadlineExceeded),
			expectedStatus: "TIMEOUT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			f := NewFormatter(false, false, &buf)

			summary := LoopSummary{
				Iterations: 5,
				TotalCost:  0.25,
				Duration:   2 * time.Minute,
				Completed:  false,
				Error:      tt.err,
				SessionID:  "test123",
			}

			f.PrintLoopSummary(summary)
			output := buf.String()

			if !strings.Contains(output, tt.expectedStatus) {
				t.Errorf("expected output to show %s for wrapped error, got: %s", tt.expectedStatus, output)
			}
		})
	}
}
