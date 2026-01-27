// Package config provides configuration management for orbit.
package config

import (
	"errors"
	"time"
)

// Config holds the configuration for an orbit execution session.
type Config struct {
	// SpecPath is the path to the specification file (required).
	SpecPath string

	// MaxIterations is the maximum number of loop iterations (default: 50).
	MaxIterations int

	// CompletionPromise is the string that signals task completion (default: "<promise>COMPLETE</promise>").
	CompletionPromise string

	// Model specifies which Claude model to use for execution (default: "opus").
	Model string

	// CheckerModel specifies which Claude model to use for completion checking (default: "haiku").
	CheckerModel string

	// MaxBudget is the maximum allowed spend in dollars (default: 100.00).
	MaxBudget float64

	// WorkingDir is the directory where orbit executes (default: ".").
	WorkingDir string

	// Verbose enables detailed output.
	Verbose bool

	// Debug enables raw JSON output streaming.
	Debug bool

	// ShowUnhandled outputs raw JSON for unhandled event types.
	ShowUnhandled bool

	// DryRun enables dry-run mode without executing commands.
	DryRun bool

	// SessionID is a unique identifier for the session.
	SessionID string

	// IterationTimeout is the maximum duration for a single iteration (default: 30m).
	IterationTimeout time.Duration

	// SystemPrompt is appended to Claude's system prompt via --append-system-prompt.
	// Contains methodology, skills, and rules that persist across iterations.
	SystemPrompt string

	// MaxTurns limits the number of agentic turns per iteration (default: 0 = unlimited).
	MaxTurns int

	// Agents is a JSON string defining custom agents for Claude CLI --agents flag.
	Agents string

	// DangerouslySkipPermissions enables the --dangerously-skip-permissions flag
	// for Claude CLI. When false (default), Claude will prompt for permission before
	// executing potentially dangerous operations. Set to true only in trusted environments.
	DangerouslySkipPermissions bool

	// MaxOutputSize is the maximum size of output to retain in bytes (default: 10MB).
	// When exceeded, older output is truncated to preserve the most recent content
	// where completion promises typically appear. Set to 0 to disable truncation.
	MaxOutputSize int

	// Theme is the colour theme for the TUI: "auto", "dark", or "light".
	// "auto" detects the terminal background colour automatically.
	// Default: "auto".
	Theme string
}

// DefaultMaxOutputSize is the default maximum output size in bytes (10MB).
const DefaultMaxOutputSize = 10 * 1024 * 1024

// DefaultContextWindow is the default context window size for unknown models.
const DefaultContextWindow = 200000

// ModelContextWindows maps model names to their context window sizes.
var ModelContextWindows = map[string]int{
	"opus":   200000,
	"sonnet": 200000,
	"haiku":  200000,
}

// GetContextWindow returns the context window size for the given model.
// Returns DefaultContextWindow for unknown models.
func GetContextWindow(model string) int {
	if window, ok := ModelContextWindows[model]; ok {
		return window
	}
	return DefaultContextWindow
}

// NewConfig returns a new Config with default values.
func NewConfig() *Config {
	return &Config{
		MaxIterations:     50,
		CompletionPromise: "<promise>COMPLETE</promise>",
		Model:             "opus",
		CheckerModel:      "haiku",
		MaxBudget:         100.00,
		WorkingDir:        ".",
		IterationTimeout:  5 * time.Minute,
		MaxOutputSize:     DefaultMaxOutputSize,
		Theme:             "auto",
	}
}

// Validate checks that the configuration is valid.
// Returns an error if validation fails.
func (c *Config) Validate() error {
	if c.SpecPath == "" {
		return errors.New("spec path is required")
	}
	if c.CompletionPromise == "" {
		return errors.New("completion promise cannot be empty")
	}
	if c.MaxIterations <= 0 {
		return errors.New("max iterations must be positive")
	}
	if c.MaxBudget <= 0 {
		return errors.New("max budget must be positive")
	}
	if c.IterationTimeout <= 0 {
		return errors.New("iteration timeout must be positive")
	}
	return nil
}
