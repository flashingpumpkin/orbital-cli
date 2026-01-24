package output

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
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

func TestPrintBanner(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	f.PrintBanner("spec.md", "sonnet", 50, "<promise>COMPLETE</promise>")

	output := buf.String()

	// Check for version string
	if !strings.Contains(output, "orbit-cli v0.1.0") {
		t.Error("expected output to contain 'orbit-cli v0.1.0'")
	}
	if !strings.Contains(output, "Autonomous Claude Code Loop") {
		t.Error("expected output to contain 'Autonomous Claude Code Loop'")
	}

	// Check for config summary
	if !strings.Contains(output, "spec.md") {
		t.Error("expected output to contain spec path")
	}
	if !strings.Contains(output, "sonnet") {
		t.Error("expected output to contain model")
	}
	if !strings.Contains(output, "50") {
		t.Error("expected output to contain max iterations")
	}
	if !strings.Contains(output, "<promise>COMPLETE</promise>") {
		t.Error("expected output to contain promise")
	}
}

func TestPrintBanner_QuietMode(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, true, &buf)

	f.PrintBanner("spec.md", "sonnet", 50, "<promise>COMPLETE</promise>")

	output := buf.String()

	// In quiet mode, banner should not be printed
	if output != "" {
		t.Errorf("expected no output in quiet mode, got: %q", output)
	}
}

func TestPrintIterationStart(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	f.PrintIterationStart(1, 10)

	output := buf.String()

	if !strings.Contains(output, "[1/10]") {
		t.Errorf("expected output to contain '[1/10]', got: %q", output)
	}
	if !strings.Contains(output, "Starting iteration") {
		t.Errorf("expected output to contain 'Starting iteration', got: %q", output)
	}
}

func TestPrintIterationStart_QuietMode(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, true, &buf)

	f.PrintIterationStart(1, 10)

	output := buf.String()

	if output != "" {
		t.Errorf("expected no output in quiet mode, got: %q", output)
	}
}

func TestPrintIterationEnd(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	f.PrintIterationEnd(30*time.Second, 1000, 500, 0.05, "COMPLETE")

	output := buf.String()

	// Check for duration
	if !strings.Contains(output, "30") {
		t.Errorf("expected output to contain duration, got: %q", output)
	}

	// Check for tokens
	if !strings.Contains(output, "1000") {
		t.Errorf("expected output to contain input tokens, got: %q", output)
	}
	if !strings.Contains(output, "500") {
		t.Errorf("expected output to contain output tokens, got: %q", output)
	}

	// Check for cost
	if !strings.Contains(output, "0.05") {
		t.Errorf("expected output to contain cost, got: %q", output)
	}

	// Check for status
	if !strings.Contains(output, "COMPLETE") {
		t.Errorf("expected output to contain status, got: %q", output)
	}
}

func TestPrintIterationEnd_QuietMode(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, true, &buf)

	f.PrintIterationEnd(30*time.Second, 1000, 500, 0.05, "COMPLETE")

	output := buf.String()

	if output != "" {
		t.Errorf("expected no output in quiet mode, got: %q", output)
	}
}

func TestPrintIterationEnd_Statuses(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"COMPLETE", "COMPLETE"},
		{"Continuing", "Continuing"},
		{"Error", "Error"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			var buf bytes.Buffer
			f := NewFormatter(false, false, &buf)

			f.PrintIterationEnd(30*time.Second, 1000, 500, 0.05, tt.status)

			output := buf.String()
			if !strings.Contains(output, tt.expected) {
				t.Errorf("expected output to contain %q, got: %q", tt.expected, output)
			}
		})
	}
}

func TestPrintSummary(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	f.PrintSummary(5, 2*time.Minute, 0.25, 5000, 2500, true)

	output := buf.String()

	// Check for separator line (should contain dashes or similar)
	if !strings.Contains(output, "---") && !strings.Contains(output, "===") {
		t.Error("expected output to contain separator line")
	}

	// Check for completion status
	if !strings.Contains(output, "Completed") || !strings.Contains(output, "Success") {
		// One of these should be present for completed=true
		if !strings.Contains(strings.ToLower(output), "complete") && !strings.Contains(strings.ToLower(output), "success") {
			t.Error("expected output to indicate completion status")
		}
	}

	// Check for totals
	if !strings.Contains(output, "5") {
		t.Error("expected output to contain iteration count")
	}
	if !strings.Contains(output, "0.25") {
		t.Error("expected output to contain total cost")
	}
	if !strings.Contains(output, "5000") {
		t.Error("expected output to contain input tokens")
	}
	if !strings.Contains(output, "2500") {
		t.Error("expected output to contain output tokens")
	}
}

func TestPrintSummary_NotCompleted(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	f.PrintSummary(5, 2*time.Minute, 0.25, 5000, 2500, false)

	output := buf.String()

	// Should indicate not completed
	lowerOutput := strings.ToLower(output)
	if !strings.Contains(lowerOutput, "not complete") && !strings.Contains(lowerOutput, "incomplete") && !strings.Contains(lowerOutput, "failed") {
		t.Error("expected output to indicate task was not completed")
	}
}

func TestPrintSummary_QuietMode(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, true, &buf)

	f.PrintSummary(5, 2*time.Minute, 0.25, 5000, 2500, true)

	output := buf.String()

	// In quiet mode, summary should still be printed (it's important info)
	// but we check it's at least minimal
	if output == "" {
		t.Error("expected some output even in quiet mode for summary")
	}
}

func TestStartSpinner(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	// Starting spinner should not panic
	f.StartSpinner("Processing...")

	// Should be able to stop it
	f.StopSpinner()
}

func TestStopSpinner_WhenNotStarted(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	// Stopping spinner when not started should not panic
	f.StopSpinner()
}

func TestStartSpinner_QuietMode(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, true, &buf)

	// In quiet mode, spinner should not start (no panic)
	f.StartSpinner("Processing...")
	f.StopSpinner()
}

func TestStartSpinner_MultipleStarts(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	// Multiple starts should not panic (should stop previous)
	f.StartSpinner("First...")
	f.StartSpinner("Second...")
	f.StopSpinner()
}

func TestFormatter_WriterOutput(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	f.PrintIterationStart(1, 5)

	if buf.Len() == 0 {
		t.Error("expected output to be written to the provided writer")
	}
}

func TestPrintBanner_ContainsSeparator(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	f.PrintBanner("spec.md", "sonnet", 50, "<promise>COMPLETE</promise>")

	output := buf.String()

	// Should contain some form of separator
	if !strings.Contains(output, "---") && !strings.Contains(output, "===") && !strings.Contains(output, "___") {
		t.Error("expected banner to contain separator line")
	}
}

func TestPrintIterationEnd_DurationFormat(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	// Test with various durations
	f.PrintIterationEnd(90*time.Second, 1000, 500, 0.05, "COMPLETE")

	output := buf.String()

	// Should contain readable duration (1m30s or similar)
	if !strings.Contains(output, "1m") && !strings.Contains(output, "90") {
		t.Errorf("expected output to contain formatted duration, got: %q", output)
	}
}

func TestPrintSummary_DurationFormat(t *testing.T) {
	var buf bytes.Buffer
	f := NewFormatter(false, false, &buf)

	f.PrintSummary(5, 5*time.Minute+30*time.Second, 0.25, 5000, 2500, true)

	output := buf.String()

	// Should contain readable duration
	if !strings.Contains(output, "5m") && !strings.Contains(output, "330") {
		t.Errorf("expected output to contain formatted duration, got: %q", output)
	}
}
