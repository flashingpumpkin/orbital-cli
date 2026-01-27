package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/flashingpumpkin/orbital/internal/completion"
	"github.com/flashingpumpkin/orbital/internal/config"
	"github.com/flashingpumpkin/orbital/internal/executor"
	"github.com/flashingpumpkin/orbital/internal/loop"
	"github.com/flashingpumpkin/orbital/internal/output"
	"github.com/flashingpumpkin/orbital/internal/session"
	"github.com/flashingpumpkin/orbital/internal/spec"
	"github.com/flashingpumpkin/orbital/internal/state"
	"github.com/flashingpumpkin/orbital/internal/tui/selector"
)

var continueCmd = &cobra.Command{
	Use:   "continue",
	Short: "Resume a terminated session",
	Long: `Resume a previously terminated orbital session.

This command loads the session state from .orbital/state/ and resumes
execution with the same session ID. Use this after:
- Ctrl+C interruption
- Unexpected termination
- System restart

If a orbital instance is already running, an error is returned.`,
	Args: cobra.NoArgs,
	RunE: runContinue,
}

func newContinueCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "continue",
		Short: "Resume a terminated session",
		Long: `Resume a previously terminated orbital session.

This command loads the session state from .orbital/state/ and resumes
execution with the same session ID. Use this after:
- Ctrl+C interruption
- Unexpected termination
- System restart

If a orbital instance is already running, an error is returned.`,
		Args: cobra.NoArgs,
		RunE: runContinue,
	}
}

func runContinue(cmd *cobra.Command, args []string) error {
	// Get current working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Collect all sessions (valid and invalid)
	collector := session.NewCollector(wd)
	sessions, err := collector.Collect()
	if err != nil {
		return fmt.Errorf("failed to collect sessions: %w", err)
	}

	if len(sessions) == 0 {
		return fmt.Errorf("no session to continue in this directory")
	}

	// Select session based on flags or interactive TUI
	selected, _, selectErr := selectSession(sessions, collector)

	if selectErr != nil {
		return selectErr
	}

	// Extract session details for resumption
	var st *state.State
	var files []string
	var sessID string
	effectiveWorkingDir := wd

	// Regular session
	st = selected.RegularState
	files = selected.SpecFiles
	sessID = selected.ID

	// Also check for queued files to merge
	stateDir := state.StateDir(wd)
	queue, err := state.LoadQueue(stateDir)
	if err == nil && !queue.IsEmpty() {
		queuedFiles, popErr := queue.Pop()
		if popErr != nil {
			return fmt.Errorf("failed to pop queued files: %w", popErr)
		}
		files = append(files, queuedFiles...)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Found %d queued file(s)\n", len(queuedFiles))
	}

	// If no files from state or queue, nothing to continue
	if len(files) == 0 {
		return fmt.Errorf("no session to continue in this directory (no active or queued files)")
	}

	// If no existing state, create a new one
	if st == nil {
		var err error
		sessID, err = generateSessionID()
		if err != nil {
			return fmt.Errorf("failed to generate session ID: %w", err)
		}
		st = state.NewState(sessID, wd, files, "", nil)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Starting new session %s with %d file(s)...\n", sessID, len(files))
	} else {
		// Update state with merged files
		st.ActiveFiles = files
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Resuming session %s with %d file(s)...\n", sessID, len(files))
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout())

	// Verbose is default, quiet suppresses it
	verbose := !quiet

	// Create config from flags (reuse root command flags)
	// Note: Only use sessionID if explicitly provided via --session-id flag
	// to resume an existing Claude conversation. Don't use orbit's internal state ID.
	// Use effectiveWorkingDir (worktree path if resuming worktree, else wd)
	cfg := &config.Config{
		SpecPath:                   files[0],
		MaxIterations:              iterations,
		CompletionPromise:          promise,
		Model:                      model,
		CheckerModel:               checkerModel,
		MaxBudget:                  budget,
		WorkingDir:                 effectiveWorkingDir,
		Verbose:                    verbose,
		Debug:                      debug,
		ShowUnhandled:              showUnhandled,
		DryRun:                     dryRun,
		SessionID:                  sessionID, // Only if user provided --session-id
		IterationTimeout:           timeout,
		MaxTurns:                   maxTurns,
		DangerouslySkipPermissions: dangerous,
		MaxOutputSize:              maxOutputSize,
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	// Load optional config file
	var fileConfig *config.FileConfig
	if configFile != "" {
		fileConfig, err = config.LoadFileConfigFrom(configFile)
		if err != nil {
			return fmt.Errorf("failed to load config file %s: %w", configFile, err)
		}
		if fileConfig == nil {
			return fmt.Errorf("config file not found: %s", configFile)
		}
	} else {
		fileConfig, err = config.LoadFileConfig(wd)
		if err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
	}
	if fileConfig != nil && fileConfig.Prompt != "" {
		spec.PromptTemplate = fileConfig.Prompt
	}

	// Handle dangerous mode: CLI flag takes precedence over config file
	// If neither is set, default is false (safe mode)
	if !dangerous && fileConfig != nil && fileConfig.Dangerous {
		cfg.DangerouslySkipPermissions = true
	}

	// Warn if dangerous mode is enabled
	if cfg.DangerouslySkipPermissions {
		fmt.Fprintln(os.Stderr, "WARNING: Running with --dangerous flag. Claude can execute commands without permission prompts.")
	}

	// Set completion promise for prompt template
	spec.CompletionPromise = cfg.CompletionPromise

	// Restore notes file from state, or generate new one
	if st.NotesFile != "" {
		spec.NotesFile = st.NotesFile
	} else if notesFile != "" {
		spec.NotesFile = notesFile
	} else {
		spec.NotesFile = generateNotesFilePath(files[0])
	}

	// Sanitise notes file path to prevent directory traversal (including via symlinks)
	absNotesPath, err := filepath.Abs(spec.NotesFile)
	if err != nil {
		return fmt.Errorf("invalid notes file path: %w", err)
	}
	absWorkingDir, err := filepath.Abs(effectiveWorkingDir)
	if err != nil {
		return fmt.Errorf("invalid working directory: %w", err)
	}
	// Resolve symlinks to prevent bypass attacks
	// For working dir, it must exist so we can resolve it
	realWorkingDir, err := filepath.EvalSymlinks(absWorkingDir)
	if err != nil {
		return fmt.Errorf("failed to resolve working directory: %w", err)
	}
	// For notes path, resolve parent dir (file may not exist yet)
	notesDir := filepath.Dir(absNotesPath)
	if _, err := os.Stat(notesDir); err == nil {
		realNotesDir, err := filepath.EvalSymlinks(notesDir)
		if err != nil {
			return fmt.Errorf("failed to resolve notes directory: %w", err)
		}
		absNotesPath = filepath.Join(realNotesDir, filepath.Base(absNotesPath))
	}
	// Ensure notes file is within the working directory
	if !strings.HasPrefix(absNotesPath, realWorkingDir+string(filepath.Separator)) && absNotesPath != realWorkingDir {
		return fmt.Errorf("notes file path must be within working directory: %s is outside %s", spec.NotesFile, effectiveWorkingDir)
	}
	spec.NotesFile = absNotesPath

	// Ensure notes directory exists (recalculate after symlink resolution)
	notesDir = filepath.Dir(spec.NotesFile)
	if err := os.MkdirAll(notesDir, 0755); err != nil {
		return fmt.Errorf("failed to create notes directory %s: %w", notesDir, err)
	}

	// Set system prompt (from flag or build default)
	if systemPrompt != "" {
		cfg.SystemPrompt = systemPrompt
	} else {
		cfg.SystemPrompt = spec.BuildSystemPrompt()
	}

	// Validate spec files exist
	sp, err := spec.Validate(files)
	if err != nil {
		return fmt.Errorf("failed to validate specs: %w", err)
	}

	// Create completion detector
	detector := completion.New(cfg.CompletionPromise)

	// Create executor with resume flag
	exec := executor.New(cfg)

	// Enable streaming output
	if cfg.Debug {
		exec.SetStreamWriter(os.Stdout)
	} else if cfg.Verbose || cfg.ShowUnhandled || todosOnly {
		streamProcessor := output.NewStreamProcessor(os.Stdout)
		if cfg.ShowUnhandled {
			streamProcessor.SetShowUnhandled(true)
		}
		if todosOnly {
			streamProcessor.SetTodosOnly(true)
		}
		exec.SetStreamWriter(streamProcessor)
	}

	// Create loop controller
	controller := loop.New(cfg, exec, detector)

	// Update state with new PID
	st.PID = os.Getpid()
	st.StartedAt = time.Now()
	if err := st.Save(); err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	// Set up state manager for queue checking after completion
	sm, err := newStateManagerAdapter(st, sp)
	if err != nil {
		return fmt.Errorf("failed to create state manager: %w", err)
	}
	controller.SetStateManager(sm)

	// Set iteration callback to update state after each iteration
	controller.SetIterationCallback(func(iteration int, totalCost float64, totalTokensIn, totalTokensOut int) error {
		return updateState(st, iteration, totalCost)
	})

	// Resolve workflow from flag or config
	wf, err := resolveWorkflow(workflowFlag, fileConfig)
	if err != nil {
		return fmt.Errorf("failed to resolve workflow: %w", err)
	}

	// Create formatter for output
	formatter := output.NewFormatter(cfg.Verbose, quiet, os.Stdout)

	// Print banner with config summary (use context files from state if available)
	printBanner(formatter, cfg, sp, st.ContextFiles, wf)

	// Build the prompt
	prompt := sp.BuildPrompt()

	// Print the command that will be executed
	if cfg.Verbose {
		fmt.Println("Command:")
		fmt.Printf("  %s\n", exec.GetCommand(prompt))
		fmt.Println()
	}

	// Create context with signal handling for graceful shutdown
	ctx, cancel := setupSignalHandler()
	defer cancel()

	// Run the loop
	loopState, err := controller.Run(ctx, prompt)

	// Print summary
	printSummary(formatter, loopState, sessID)

	// Handle state cleanup or preservation
	if err != nil {
		// Use errors.Is() to handle wrapped errors correctly
		switch {
		case errors.Is(err, loop.ErrMaxIterationsReached):
			os.Exit(1)
		case errors.Is(err, loop.ErrBudgetExceeded):
			os.Exit(2)
		case errors.Is(err, context.DeadlineExceeded):
			os.Exit(3)
		case errors.Is(err, context.Canceled):
			// Summary already printed above with resume instructions
			os.Exit(130)
		default:
			os.Exit(4)
		}
	}

	// On successful completion, clean up state
	if err := cleanupState(st); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to cleanup state: %v\n", err)
	}

	return nil
}

// sessionValidator provides the ValidSessions method for filtering sessions.
type sessionValidator interface {
	ValidSessions(sessions []session.Session) []session.Session
}

// selectSession handles session selection.
// Priority:
// 1. Single valid session: auto-select
// 2. Multiple sessions: TUI selector (unless --non-interactive)
func selectSession(sessions []session.Session, validator sessionValidator) (*session.Session, []string, error) {
	validSessions := validator.ValidSessions(sessions)

	// Auto-resume if only one valid session
	if len(validSessions) == 1 {
		return &validSessions[0], nil, nil
	}

	// No valid sessions
	if len(validSessions) == 0 {
		if nonInteractive {
			return nil, nil, fmt.Errorf("no valid sessions to resume")
		}
	}

	// Multiple sessions - use TUI selector
	if nonInteractive {
		return nil, nil, fmt.Errorf("multiple sessions found:\n%s", formatSessionList(sessions))
	}

	// Run TUI selector
	result, err := selector.Run(sessions)
	if err != nil {
		return nil, nil, fmt.Errorf("session selection failed: %w", err)
	}

	if result.Cancelled {
		return nil, result.CleanupPaths, fmt.Errorf("selection cancelled")
	}

	if result.Session == nil {
		return nil, result.CleanupPaths, fmt.Errorf("no session selected")
	}

	return result.Session, result.CleanupPaths, nil
}

// formatSessionList formats sessions for display in error messages.
func formatSessionList(sessions []session.Session) string {
	var result string
	for i, s := range sessions {
		status := "valid"
		if !s.Valid {
			status = "invalid: " + s.InvalidReason
		}
		result += fmt.Sprintf("  [%d] %s - %s\n", i+1, s.DisplayName(), status)
	}
	return result
}
