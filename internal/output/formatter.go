// Package output provides formatting utilities for orbit output.
package output

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
)

// Formatter handles formatted output for orbit.
type Formatter struct {
	verbose bool
	quiet   bool
	noColor bool
	writer  io.Writer
	spinner *spinner.Spinner
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
	Duration    time.Duration
	Completed   bool
	Error       error
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

// PrintBanner prints the orbit banner with configuration summary.
func (f *Formatter) PrintBanner(specPath, model string, maxIterations int, promise string) {
	if f.quiet {
		return
	}

	cyan := color.New(color.FgCyan, color.Bold)
	white := color.New(color.FgWhite)

	_, _ = cyan.Fprintln(f.writer, "orbit-cli v0.1.0 - Autonomous Claude Code Loop")
	_, _ = fmt.Fprintln(f.writer, "")
	_, _ = white.Fprintf(f.writer, "  Spec:       %s\n", specPath)
	_, _ = white.Fprintf(f.writer, "  Model:      %s\n", model)
	_, _ = white.Fprintf(f.writer, "  Iterations: %d\n", maxIterations)
	_, _ = white.Fprintf(f.writer, "  Promise:    %s\n", promise)
	_, _ = fmt.Fprintln(f.writer, "")
	_, _ = fmt.Fprintln(f.writer, "---------------------------------------------------")
	_, _ = fmt.Fprintln(f.writer, "")
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
	red := color.New(color.FgRed, color.Bold)

	_, _ = fmt.Fprintln(f.writer, "")
	_, _ = cyan.Fprintln(f.writer, "════════════════════════════════════════════════════════════════")
	_, _ = cyan.Fprintln(f.writer, "                           Summary                              ")
	_, _ = cyan.Fprintln(f.writer, "════════════════════════════════════════════════════════════════")
	_, _ = white.Fprintf(f.writer, "  Iterations:  %d\n", summary.Iterations)
	_, _ = white.Fprintf(f.writer, "  Total Cost:  $%.4f USD\n", summary.TotalCost)
	_, _ = white.Fprintf(f.writer, "  Total Tokens: %d\n", summary.TotalTokens)
	_, _ = white.Fprintf(f.writer, "  Duration:    %v\n", formatDuration(summary.Duration))

	if summary.Completed {
		_, _ = green.Fprintln(f.writer, "  Status:      COMPLETED (promise detected)")
	} else if summary.Error != nil {
		_, _ = red.Fprintf(f.writer, "  Status:      TERMINATED (%v)\n", summary.Error)
	} else {
		_, _ = red.Fprintln(f.writer, "  Status:      NOT COMPLETED")
	}

	_, _ = fmt.Fprintln(f.writer, "")
}

// PrintIterationStart prints the start of an iteration.
func (f *Formatter) PrintIterationStart(current, total int) {
	if f.quiet {
		return
	}

	blue := color.New(color.FgBlue, color.Bold)
	_, _ = blue.Fprintf(f.writer, "[%d/%d] Starting iteration...\n", current, total)
}

// PrintIterationEnd prints the end of an iteration with statistics.
func (f *Formatter) PrintIterationEnd(duration time.Duration, tokensIn, tokensOut int, cost float64, status string) {
	if f.quiet {
		return
	}

	white := color.New(color.FgWhite)
	_, _ = white.Fprintf(f.writer, "  Duration: %s | Tokens: %d in, %d out | Cost: $%.4f\n",
		formatDuration(duration), tokensIn, tokensOut, cost)

	// Print status with appropriate colour
	var statusColor *color.Color
	switch status {
	case "COMPLETE":
		statusColor = color.New(color.FgGreen, color.Bold)
	case "Continuing":
		statusColor = color.New(color.FgYellow)
	default:
		statusColor = color.New(color.FgRed)
	}

	_, _ = statusColor.Fprintf(f.writer, "  Status: %s\n", status)
	_, _ = fmt.Fprintln(f.writer, "")
}

// PrintSummary prints the final summary of the orbit execution.
func (f *Formatter) PrintSummary(iterations int, duration time.Duration, cost float64, tokensIn, tokensOut int, completed bool) {
	_, _ = fmt.Fprintln(f.writer, "---------------------------------------------------")
	_, _ = fmt.Fprintln(f.writer, "")

	cyan := color.New(color.FgCyan, color.Bold)
	_, _ = cyan.Fprintln(f.writer, "Summary")
	_, _ = fmt.Fprintln(f.writer, "")

	white := color.New(color.FgWhite)
	_, _ = white.Fprintf(f.writer, "  Iterations:   %d\n", iterations)
	_, _ = white.Fprintf(f.writer, "  Duration:     %s\n", formatDuration(duration))
	_, _ = white.Fprintf(f.writer, "  Total Cost:   $%.4f\n", cost)
	_, _ = white.Fprintf(f.writer, "  Tokens In:    %d\n", tokensIn)
	_, _ = white.Fprintf(f.writer, "  Tokens Out:   %d\n", tokensOut)
	_, _ = fmt.Fprintln(f.writer, "")

	if completed {
		green := color.New(color.FgGreen, color.Bold)
		_, _ = green.Fprintln(f.writer, "  Status: Completed Successfully")
	} else {
		red := color.New(color.FgRed, color.Bold)
		_, _ = red.Fprintln(f.writer, "  Status: Not Completed")
	}

	_, _ = fmt.Fprintln(f.writer, "")
}

// StartSpinner starts a progress spinner with the given message.
func (f *Formatter) StartSpinner(message string) {
	if f.quiet {
		return
	}

	// Stop any existing spinner
	if f.spinner != nil {
		f.spinner.Stop()
	}

	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Writer = f.writer
	s.Suffix = " " + message

	if f.noColor {
		_ = s.Color("reset")
	} else {
		_ = s.Color("cyan")
	}

	f.spinner = s
	f.spinner.Start()
}

// StopSpinner stops the progress spinner.
func (f *Formatter) StopSpinner() {
	if f.spinner != nil {
		f.spinner.Stop()
		f.spinner = nil
	}
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

// WorktreeSetupConfig contains configuration for worktree setup display.
type WorktreeSetupConfig struct {
	OriginalBranch string
	Model          string
	SpecFile       string
}

// PrintWorktreeSetupStart prints the worktree setup header.
func (f *Formatter) PrintWorktreeSetupStart(cfg WorktreeSetupConfig) {
	if f.quiet {
		return
	}

	cyan := color.New(color.FgCyan, color.Bold)
	white := color.New(color.FgWhite)
	dim := color.New(color.FgHiBlack)

	_, _ = fmt.Fprintln(f.writer, "")
	_, _ = cyan.Fprintln(f.writer, "┌─────────────────────────────────────────────────────────────────┐")
	_, _ = cyan.Fprintln(f.writer, "│                    Worktree Setup Phase                         │")
	_, _ = cyan.Fprintln(f.writer, "└─────────────────────────────────────────────────────────────────┘")
	_, _ = fmt.Fprintln(f.writer, "")
	_, _ = white.Fprintf(f.writer, "  Branch:      %s\n", cfg.OriginalBranch)
	_, _ = white.Fprintf(f.writer, "  Model:       %s\n", cfg.Model)
	_, _ = dim.Fprintf(f.writer, "  Spec:        %s\n", cfg.SpecFile)
	_, _ = fmt.Fprintln(f.writer, "")
}

// StartWorktreeSetupSpinner starts the worktree setup spinner.
func (f *Formatter) StartWorktreeSetupSpinner() {
	f.StartSpinner("Creating isolated worktree...")
}

// WorktreeSetupResult contains result of worktree setup.
type WorktreeSetupResult struct {
	WorktreePath string
	BranchName   string
	Cost         float64
	TokensIn     int
	TokensOut    int
	Duration     time.Duration
}

// PrintWorktreeSetupComplete prints the worktree setup completion.
func (f *Formatter) PrintWorktreeSetupComplete(result WorktreeSetupResult) {
	f.StopSpinner()

	if f.quiet {
		return
	}

	green := color.New(color.FgGreen, color.Bold)
	white := color.New(color.FgWhite)
	dim := color.New(color.FgHiBlack)

	_, _ = green.Fprintln(f.writer, "  ✓ Worktree created successfully")
	_, _ = fmt.Fprintln(f.writer, "")
	_, _ = white.Fprintf(f.writer, "  Path:        %s\n", result.WorktreePath)
	_, _ = white.Fprintf(f.writer, "  Branch:      %s\n", result.BranchName)
	_, _ = dim.Fprintf(f.writer, "  Duration:    %s\n", formatDuration(result.Duration))
	_, _ = dim.Fprintf(f.writer, "  Cost:        $%.4f | %d tokens\n", result.Cost, result.TokensIn+result.TokensOut)
	_, _ = fmt.Fprintln(f.writer, "")
}

// PrintWorktreeSetupError prints a worktree setup error.
func (f *Formatter) PrintWorktreeSetupError(err error) {
	f.StopSpinner()

	red := color.New(color.FgRed, color.Bold)
	_, _ = red.Fprintf(f.writer, "  ✗ Worktree setup failed: %v\n", err)
	_, _ = fmt.Fprintln(f.writer, "")
}

// PrintWorktreeMergeStart prints the worktree merge phase header.
func (f *Formatter) PrintWorktreeMergeStart(worktreePath, branchName, originalBranch string) {
	if f.quiet {
		return
	}

	cyan := color.New(color.FgCyan, color.Bold)
	white := color.New(color.FgWhite)

	_, _ = fmt.Fprintln(f.writer, "")
	_, _ = cyan.Fprintln(f.writer, "┌─────────────────────────────────────────────────────────────────┐")
	_, _ = cyan.Fprintln(f.writer, "│                    Worktree Merge Phase                         │")
	_, _ = cyan.Fprintln(f.writer, "└─────────────────────────────────────────────────────────────────┘")
	_, _ = fmt.Fprintln(f.writer, "")
	_, _ = white.Fprintf(f.writer, "  Merging:     %s -> %s\n", branchName, originalBranch)
	_, _ = fmt.Fprintln(f.writer, "")
}

// StartWorktreeMergeSpinner starts the worktree merge spinner.
func (f *Formatter) StartWorktreeMergeSpinner() {
	f.StartSpinner("Rebasing and merging changes...")
}

// PrintWorktreeMergeComplete prints the worktree merge completion.
func (f *Formatter) PrintWorktreeMergeComplete(cost float64, tokensIn, tokensOut int) {
	f.StopSpinner()

	if f.quiet {
		return
	}

	green := color.New(color.FgGreen, color.Bold)
	dim := color.New(color.FgHiBlack)

	_, _ = green.Fprintln(f.writer, "  ✓ Merge completed successfully")
	_, _ = dim.Fprintf(f.writer, "  Cost:        $%.4f | %d tokens\n", cost, tokensIn+tokensOut)
	_, _ = fmt.Fprintln(f.writer, "")
}

// PrintWorktreeCleanupComplete prints worktree cleanup success.
func (f *Formatter) PrintWorktreeCleanupComplete() {
	if f.quiet {
		return
	}

	green := color.New(color.FgGreen)
	_, _ = green.Fprintln(f.writer, "  ✓ Worktree and branch cleaned up")
	_, _ = fmt.Fprintln(f.writer, "")
}

// PrintWorktreePreserved prints info about a preserved worktree.
func (f *Formatter) PrintWorktreePreserved(worktreePath string) {
	yellow := color.New(color.FgYellow, color.Bold)
	white := color.New(color.FgWhite)

	_, _ = fmt.Fprintln(f.writer, "")
	_, _ = yellow.Fprintln(f.writer, "  Worktree preserved for resume:")
	_, _ = white.Fprintf(f.writer, "  Path:        %s\n", worktreePath)
	_, _ = white.Fprintln(f.writer, "  Resume:      orbit-cli continue")
	_, _ = fmt.Fprintln(f.writer, "")
}
