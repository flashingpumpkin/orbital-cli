// Package executor provides functionality to execute Claude CLI commands.
package executor

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/flashingpumpkin/orbital/internal/config"
	"github.com/flashingpumpkin/orbital/internal/output"
)

const (
	// scannerInitialBufSize is the initial buffer size for the scanner (64KB).
	scannerInitialBufSize = 64 * 1024

	// scannerMaxBufSize is the maximum line size the scanner can handle (10MB).
	// Claude's stream-json output can have single lines >1MB for large file reads.
	scannerMaxBufSize = 10 * 1024 * 1024

	// scannerWarnThreshold is the size threshold above which a warning is logged (8MB).
	scannerWarnThreshold = 8 * 1024 * 1024

	// truncationMarker is the message prepended to truncated output.
	truncationMarker = "[OUTPUT TRUNCATED - SHOWING MOST RECENT CONTENT]\n"
)

// ExecutionResult contains the result of a Claude CLI execution.
type ExecutionResult struct {
	// Output is the captured stdout from the Claude process.
	Output string

	// ExitCode is the exit code of the Claude process.
	ExitCode int

	// Duration is how long the execution took.
	Duration time.Duration

	// TokensIn is the number of input tokens used during execution.
	TokensIn int

	// TokensOut is the number of output tokens used during execution.
	TokensOut int

	// CostUSD is the estimated cost in USD for the execution.
	CostUSD float64

	// Completed indicates whether the execution completed successfully.
	Completed bool

	// Error contains any error that occurred during execution.
	Error error
}

// Executor manages the execution of Claude CLI commands.
type Executor struct {
	config       *config.Config
	claudeCmd    string
	streamWriter io.Writer
	verbose      bool
}

// New creates a new Executor with the given configuration.
func New(cfg *config.Config) *Executor {
	return &Executor{
		config:    cfg,
		claudeCmd: "claude",
		verbose:   cfg.Verbose,
	}
}

// SetStreamWriter sets the writer for streaming output.
func (e *Executor) SetStreamWriter(w io.Writer) {
	e.streamWriter = w
}

// GetCommand returns the full command string that would be executed.
func (e *Executor) GetCommand(prompt string) string {
	args := e.BuildArgs(prompt)
	// Quote args that contain spaces
	quotedArgs := make([]string, len(args))
	for i, arg := range args {
		if strings.Contains(arg, " ") || strings.Contains(arg, "\n") {
			quotedArgs[i] = fmt.Sprintf("%q", arg)
		} else {
			quotedArgs[i] = arg
		}
	}
	return e.claudeCmd + " " + strings.Join(quotedArgs, " ")
}

// BuildArgs constructs the command-line arguments for the Claude CLI.
func (e *Executor) BuildArgs(prompt string) []string {
	args := []string{
		"-p",
		"--output-format", "stream-json",
		"--verbose",
		"--model", e.config.Model,
		"--max-budget-usd", fmt.Sprintf("%.2f", e.config.MaxBudget),
	}

	// Only include --dangerously-skip-permissions when explicitly enabled
	if e.config.DangerouslySkipPermissions {
		args = append(args, "--dangerously-skip-permissions")
	}

	if e.config.SessionID != "" {
		args = append(args, "--resume", e.config.SessionID)
	}

	if e.config.SystemPrompt != "" {
		args = append(args, "--append-system-prompt", e.config.SystemPrompt)
	}

	if e.config.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", e.config.MaxTurns))
	}

	if e.config.Agents != "" {
		args = append(args, "--agents", e.config.Agents)
	}

	args = append(args, prompt)

	return args
}

// extractStats parses the raw output and extracts token counts and cost.
// Returns tokensIn, tokensOut, and costUSD.
func extractStats(rawOutput string) (int, int, float64) {
	parser := output.NewParser()
	for _, line := range strings.Split(rawOutput, "\n") {
		_, _ = parser.ParseLine([]byte(line))
	}
	stats := parser.GetStats()
	return stats.TokensIn, stats.TokensOut, stats.CostUSD
}

// truncateOutput truncates output to the specified maximum size, preserving
// the most recent content where completion promises typically appear.
// Returns the truncated output and whether truncation occurred.
func truncateOutput(content []byte, maxSize int) ([]byte, bool) {
	if maxSize <= 0 || len(content) <= maxSize {
		return content, false
	}

	// Keep approximately half the max size to avoid cutting too close
	keepSize := maxSize / 2
	truncatePoint := len(content) - keepSize

	// Find the next newline after truncatePoint to avoid cutting mid-line
	for i := truncatePoint; i < len(content); i++ {
		if content[i] == '\n' {
			truncatePoint = i + 1
			break
		}
	}

	// Build result with truncation marker
	result := make([]byte, 0, len(truncationMarker)+len(content)-truncatePoint)
	result = append(result, truncationMarker...)
	result = append(result, content[truncatePoint:]...)

	return result, true
}

// Execute runs the Claude CLI with the given prompt.
// It respects context cancellation and returns an error if Claude is not in PATH.
// If a stream writer is set, output is streamed line-by-line as it arrives.
// When WorkingDir is set in config, Claude CLI runs in that directory.
func (e *Executor) Execute(ctx context.Context, prompt string) (*ExecutionResult, error) {
	// Check if the command exists in PATH
	cmdPath, err := exec.LookPath(e.claudeCmd)
	if err != nil {
		return nil, fmt.Errorf("claude not found in PATH: %w", err)
	}

	args := e.BuildArgs(prompt)
	cmd := exec.CommandContext(ctx, cmdPath, args...)

	// Set working directory if configured (used for worktree mode)
	if e.config.WorkingDir != "" && e.config.WorkingDir != "." {
		cmd.Dir = e.config.WorkingDir
	}

	// Use pipe for streaming if writer is set, otherwise buffer
	var stdout bytes.Buffer

	if e.streamWriter != nil {
		stdoutPipe, err := cmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
		}

		startTime := time.Now()
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start command: %w", err)
		}

		// Parse output during streaming to avoid double-parsing at the end
		parser := output.NewParser()

		// Read and stream output line by line
		scanner := bufio.NewScanner(stdoutPipe)
		// Increase buffer size for long lines (10MB max to handle large file reads)
		buf := make([]byte, 0, scannerInitialBufSize)
		scanner.Buffer(buf, scannerMaxBufSize)

		// Track if truncation has occurred
		var truncated bool
		maxOutputSize := e.config.MaxOutputSize

		var scanErr error
		for scanner.Scan() {
			line := scanner.Text()
			lineLen := len(line)

			// Warn about very large lines that approach the buffer limit
			if e.verbose && lineLen > scannerWarnThreshold {
				fmt.Fprintf(os.Stderr, "warning: large output line (%d bytes), approaching %d byte limit\n",
					lineLen, scannerMaxBufSize)
			}

			stdout.WriteString(line)
			stdout.WriteString("\n")

			// Check if truncation is needed (only if MaxOutputSize > 0)
			if maxOutputSize > 0 && stdout.Len() > maxOutputSize {
				// Truncate from the front to preserve recent content
				// Keep approximately half the max size to avoid frequent truncation
				keepSize := maxOutputSize / 2
				content := stdout.Bytes()
				truncatePoint := len(content) - keepSize

				// Find the next newline after truncatePoint to avoid cutting mid-line
				for i := truncatePoint; i < len(content); i++ {
					if content[i] == '\n' {
						truncatePoint = i + 1
						break
					}
				}

				// Rebuild buffer with truncation marker and remaining content
				remaining := content[truncatePoint:]
				stdout.Reset()
				stdout.WriteString(truncationMarker)
				stdout.Write(remaining)

				// Log warning on first truncation only
				if !truncated {
					truncated = true
					if e.verbose {
						fmt.Fprintf(os.Stderr, "warning: output exceeded %d bytes, truncating to preserve recent content\n",
							maxOutputSize)
					}
				}
			}

			// Parse line for stats during streaming
			_, _ = parser.ParseLine([]byte(line))
			// Write to stream writer
			_, _ = fmt.Fprintln(e.streamWriter, line)
		}

		// Check for scanner errors (including buffer overflow)
		if err := scanner.Err(); err != nil {
			if err == bufio.ErrTooLong {
				scanErr = fmt.Errorf("output line exceeded %d byte limit: %w", scannerMaxBufSize, err)
			} else {
				scanErr = fmt.Errorf("scanner error: %w", err)
			}
		}

		runErr := cmd.Wait()
		duration := time.Since(startTime)

		// Get stats from streaming parser (already parsed, no double-parsing)
		stats := parser.GetStats()

		// Handle context cancellation
		if ctx.Err() != nil {
			return &ExecutionResult{
				Output:    stdout.String(),
				Duration:  duration,
				TokensIn:  stats.TokensIn,
				TokensOut: stats.TokensOut,
				CostUSD:   stats.CostUSD,
				Completed: false,
				Error:     ctx.Err(),
			}, ctx.Err()
		}

		// Handle scanner errors (e.g., line too long)
		if scanErr != nil {
			return &ExecutionResult{
				Output:    stdout.String(),
				Duration:  duration,
				TokensIn:  stats.TokensIn,
				TokensOut: stats.TokensOut,
				CostUSD:   stats.CostUSD,
				Completed: false,
				Error:     scanErr,
			}, scanErr
		}

		// Handle command execution error
		if runErr != nil {
			exitCode := 1
			if exitErr, ok := runErr.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
			return &ExecutionResult{
				Output:    stdout.String(),
				ExitCode:  exitCode,
				Duration:  duration,
				TokensIn:  stats.TokensIn,
				TokensOut: stats.TokensOut,
				CostUSD:   stats.CostUSD,
				Completed: false,
				Error:     runErr,
			}, nil
		}

		return &ExecutionResult{
			Output:    stdout.String(),
			ExitCode:  0,
			Duration:  duration,
			TokensIn:  stats.TokensIn,
			TokensOut: stats.TokensOut,
			CostUSD:   stats.CostUSD,
			Completed: true,
			Error:     nil,
		}, nil
	}

	// Non-streaming path: parse once at the end
	cmd.Stdout = &stdout

	startTime := time.Now()
	runErr := cmd.Run()
	duration := time.Since(startTime)

	// Parse output once for stats (parse before truncation to get accurate stats)
	tokensIn, tokensOut, cost := extractStats(stdout.String())

	// Apply truncation if configured
	outputBytes := stdout.Bytes()
	if e.config.MaxOutputSize > 0 {
		truncatedOutput, wasTruncated := truncateOutput(outputBytes, e.config.MaxOutputSize)
		if wasTruncated {
			outputBytes = truncatedOutput
			if e.verbose {
				fmt.Fprintf(os.Stderr, "warning: output exceeded %d bytes, truncating to preserve recent content\n",
					e.config.MaxOutputSize)
			}
		}
	}
	outputStr := string(outputBytes)

	// Handle context cancellation - check this first as it takes priority
	if ctx.Err() != nil {
		return &ExecutionResult{
			Output:    outputStr,
			Duration:  duration,
			TokensIn:  tokensIn,
			TokensOut: tokensOut,
			CostUSD:   cost,
			Completed: false,
			Error:     ctx.Err(),
		}, ctx.Err()
	}

	// Handle command execution error
	if runErr != nil {
		exitCode := 1
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		return &ExecutionResult{
			Output:    outputStr,
			ExitCode:  exitCode,
			Duration:  duration,
			TokensIn:  tokensIn,
			TokensOut: tokensOut,
			CostUSD:   cost,
			Completed: false,
			Error:     runErr,
		}, nil
	}

	return &ExecutionResult{
		Output:    outputStr,
		ExitCode:  0,
		Duration:  duration,
		TokensIn:  tokensIn,
		TokensOut: tokensOut,
		CostUSD:   cost,
		Completed: true,
		Error:     nil,
	}, nil
}
