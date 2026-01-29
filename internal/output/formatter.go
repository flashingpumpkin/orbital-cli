// Package output provides formatting utilities for orbit output.
package output

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/fatih/color"
	orberrors "github.com/flashingpumpkin/orbital/internal/errors"
)

// Formatter handles formatted output for orbit.
type Formatter struct {
	verbose bool
	quiet   bool
	noColor bool
	writer  io.Writer
}

// BannerConfig contains all configuration for the rich banner display.
type BannerConfig struct {
	SpecFile      string
	ContextFiles  []string
	WorkflowName  string
	WorkflowSteps int
	HasGates      bool
	Model         string
	CheckerModel  string
	MaxIterations int
	Budget        float64
	Timeout       time.Duration
	WorkingDir    string
	NotesFile     string
	SessionID     string
	DryRun        bool
	Debug         bool
}

// LoopSummary contains summary information for loop execution.
type LoopSummary struct {
	Iterations  int
	TotalCost   float64
	TotalTokens int
	TokensIn    int
	TokensOut   int
	Duration    time.Duration
	Completed   bool
	Error       error
	SessionID   string // For resume instructions on interrupt
}

// NewFormatter creates a new Formatter with the specified options.
// It checks the NO_COLOR environment variable to determine if colour output should be disabled.
func NewFormatter(verbose, quiet bool, w io.Writer) *Formatter {
	noColor := os.Getenv("NO_COLOR") != ""

	if noColor {
		color.NoColor = true
	}

	return &Formatter{
		verbose: verbose,
		quiet:   quiet,
		noColor: noColor,
		writer:  w,
	}
}

// PrintRichBanner prints the orbit banner with full configuration details.
func (f *Formatter) PrintRichBanner(cfg BannerConfig) {
	if f.quiet {
		return
	}

	cyan := color.New(color.FgCyan, color.Bold)
	white := color.New(color.FgWhite)
	dim := color.New(color.FgHiBlack)
	yellow := color.New(color.FgYellow)

	// Box header
	_, _ = cyan.Fprintln(f.writer, "╔═══════════════════════════════════════════════════════════════╗")
	_, _ = cyan.Fprintln(f.writer, "║                    Orbit - I'm learnding!                     ║")
	_, _ = cyan.Fprintln(f.writer, "╚═══════════════════════════════════════════════════════════════╝")
	_, _ = fmt.Fprintln(f.writer, "")

	// Configuration section
	_, _ = white.Fprintf(f.writer, "  Spec:        %s\n", cfg.SpecFile)

	// Context files
	if len(cfg.ContextFiles) > 0 {
		_, _ = white.Fprintf(f.writer, "  Context:     %d file(s)\n", len(cfg.ContextFiles))
		for _, path := range cfg.ContextFiles {
			_, _ = dim.Fprintf(f.writer, "               - %s\n", path)
		}
	}

	// Workflow info
	_, _ = white.Fprintf(f.writer, "  Workflow:    %s", cfg.WorkflowName)
	if cfg.HasGates {
		_, _ = dim.Fprintf(f.writer, " (%d steps, with gates)\n", cfg.WorkflowSteps)
	} else {
		_, _ = dim.Fprintf(f.writer, " (%d step)\n", cfg.WorkflowSteps)
	}

	// Models
	_, _ = white.Fprintf(f.writer, "  Model:       %s\n", cfg.Model)
	_, _ = white.Fprintf(f.writer, "  Checker:     %s\n", cfg.CheckerModel)

	// Limits
	_, _ = white.Fprintf(f.writer, "  Iterations:  max %d\n", cfg.MaxIterations)
	_, _ = white.Fprintf(f.writer, "  Budget:      $%.2f USD\n", cfg.Budget)
	_, _ = white.Fprintf(f.writer, "  Timeout:     %v per iteration\n", cfg.Timeout)

	// Paths
	_, _ = white.Fprintf(f.writer, "  Working Dir: %s\n", cfg.WorkingDir)
	_, _ = white.Fprintf(f.writer, "  Notes File:  %s\n", cfg.NotesFile)

	// Session info
	if cfg.SessionID != "" {
		_, _ = white.Fprintf(f.writer, "  Resuming:    session %s\n", cfg.SessionID)
	}

	// Special modes
	if cfg.DryRun {
		_, _ = yellow.Fprintln(f.writer, "  Mode:        DRY RUN (no commands will be executed)")
	}
	if cfg.Debug {
		_, _ = yellow.Fprintln(f.writer, "  Debug:       enabled (raw JSON output)")
	}

	_, _ = fmt.Fprintln(f.writer, "")
	_, _ = cyan.Fprintln(f.writer, "Starting loop...")
	_, _ = fmt.Fprintln(f.writer, "")
}

// PrintLoopSummary prints the final summary of loop execution.
func (f *Formatter) PrintLoopSummary(summary LoopSummary) {
	// Always print summary (even in quiet mode, it's important info)
	cyan := color.New(color.FgCyan, color.Bold)
	white := color.New(color.FgWhite)
	green := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow, color.Bold)
	red := color.New(color.FgRed, color.Bold)

	_, _ = fmt.Fprintln(f.writer, "")
	_, _ = cyan.Fprintln(f.writer, "════════════════════════════════════════════════════════════════")
	_, _ = cyan.Fprintln(f.writer, "                           Summary                              ")
	_, _ = cyan.Fprintln(f.writer, "════════════════════════════════════════════════════════════════")
	_, _ = white.Fprintf(f.writer, "  Iterations:   %d\n", summary.Iterations)
	_, _ = white.Fprintf(f.writer, "  Duration:     %v\n", formatDuration(summary.Duration))
	_, _ = white.Fprintf(f.writer, "  Cost:         $%.4f USD\n", summary.TotalCost)

	// Show detailed token breakdown if available, otherwise fall back to TotalTokens
	if summary.TokensIn > 0 || summary.TokensOut > 0 {
		_, _ = white.Fprintf(f.writer, "  Tokens:       %d in / %d out\n", summary.TokensIn, summary.TokensOut)
	} else if summary.TotalTokens > 0 {
		_, _ = white.Fprintf(f.writer, "  Tokens:       %d\n", summary.TotalTokens)
	}

	// Status line with appropriate colour
	if summary.Completed {
		_, _ = green.Fprintln(f.writer, "  Status:       COMPLETED")
	} else if summary.Error != nil {
		// Check for specific error types using errors.Is for proper wrapped error handling
		switch {
		case errors.Is(summary.Error, context.Canceled):
			_, _ = yellow.Fprintln(f.writer, "  Status:       INTERRUPTED")
		case errors.Is(summary.Error, orberrors.ErrMaxIterationsReached):
			_, _ = red.Fprintln(f.writer, "  Status:       MAX ITERATIONS REACHED")
		case errors.Is(summary.Error, orberrors.ErrBudgetExceeded):
			_, _ = red.Fprintln(f.writer, "  Status:       BUDGET EXCEEDED")
		case errors.Is(summary.Error, context.DeadlineExceeded):
			_, _ = red.Fprintln(f.writer, "  Status:       TIMEOUT")
		default:
			_, _ = red.Fprintf(f.writer, "  Status:       FAILED (%v)\n", summary.Error)
		}
	} else {
		_, _ = red.Fprintln(f.writer, "  Status:       NOT COMPLETED")
	}

	// Show resume instructions if session has a session ID and can be resumed
	// This includes interrupted sessions and other non-completed states
	if summary.SessionID != "" && !summary.Completed {
		_, _ = fmt.Fprintln(f.writer, "")
		_, _ = white.Fprintln(f.writer, "  Resume with:")
		_, _ = white.Fprintf(f.writer, "    orbital continue\n")
	}

	_, _ = fmt.Fprintln(f.writer, "")
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	if seconds == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}

// PrintStepStart prints the start of a workflow step.
func (f *Formatter) PrintStepStart(name string, position, total int) {
	if f.quiet {
		return
	}

	blue := color.New(color.FgBlue, color.Bold)
	_, _ = blue.Fprintf(f.writer, "[Step %d/%d] %s\n", position, total, name)
}

// PrintStepComplete prints the completion of a workflow step.
func (f *Formatter) PrintStepComplete(name string, duration time.Duration, cost float64, tokens int) {
	if f.quiet {
		return
	}

	white := color.New(color.FgWhite)
	_, _ = white.Fprintf(f.writer, "  Completed in %s | $%.4f | %d tokens\n", formatDuration(duration), cost, tokens)
}

// PrintGateResult prints the result of a gate check.
func (f *Formatter) PrintGateResult(passed bool, retries, maxRetries int) {
	if f.quiet {
		return
	}

	if passed {
		green := color.New(color.FgGreen)
		_, _ = green.Fprintln(f.writer, "  Gate: PASS")
	} else {
		yellow := color.New(color.FgYellow)
		_, _ = yellow.Fprintf(f.writer, "  Gate: FAIL (retry %d/%d)\n", retries+1, maxRetries)
	}
}

// StepSummary contains summary information for a completed step.
type StepSummary struct {
	Name       string
	Status     string // "passed", "failed", "completed"
	Cost       float64
	Tokens     int
	GateResult string // "PASS", "FAIL", "" for non-gate steps
}

// PrintWorkflowSummary prints a summary of all completed workflow steps.
func (f *Formatter) PrintWorkflowSummary(steps []StepSummary, totalCost float64, totalTokens int) {
	if f.quiet {
		return
	}

	_, _ = fmt.Fprintln(f.writer, "")
	cyan := color.New(color.FgCyan, color.Bold)
	_, _ = cyan.Fprintln(f.writer, "Workflow Steps Summary")
	_, _ = fmt.Fprintln(f.writer, "")

	white := color.New(color.FgWhite)
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)

	for i, step := range steps {
		var statusIcon string
		var statusColor *color.Color

		switch step.Status {
		case "passed":
			statusIcon = "✓"
			statusColor = green
		case "failed":
			statusIcon = "✗"
			statusColor = red
		default:
			statusIcon = "•"
			statusColor = white
		}

		// Print step with status
		_, _ = statusColor.Fprintf(f.writer, "  %s %d. %s", statusIcon, i+1, step.Name)

		// Add gate result if applicable
		if step.GateResult != "" {
			if step.GateResult == "PASS" {
				_, _ = green.Fprintf(f.writer, " [%s]", step.GateResult)
			} else {
				_, _ = red.Fprintf(f.writer, " [%s]", step.GateResult)
			}
		}
		_, _ = fmt.Fprintln(f.writer)

		// Print cost and tokens
		_, _ = white.Fprintf(f.writer, "      $%.4f | %d tokens\n", step.Cost, step.Tokens)
	}

	// Print totals
	_, _ = fmt.Fprintln(f.writer, "")
	_, _ = white.Fprintf(f.writer, "  Total: $%.4f | %d tokens\n", totalCost, totalTokens)
}
