package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	"github.com/flashingpumpkin/orbital/internal/worktree"
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
	selected, cleanupPaths, selectErr := selectSession(sessions, collector)

	// Handle cleanup of stale sessions (even if selection was cancelled)
	var cleanupSucceeded, cleanupFailed int
	if len(cleanupPaths) > 0 {
		wtStateManager := worktree.NewStateManager(wd)
		for _, path := range cleanupPaths {
			if err := wtStateManager.Remove(path); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to remove stale entry %s: %v\n", path, err)
				cleanupFailed++
			} else {
				fmt.Fprintf(os.Stderr, "Removed stale session: %s\n", path)
				cleanupSucceeded++
			}
		}
		// Report cleanup summary
		if cleanupFailed > 0 {
			fmt.Fprintf(os.Stderr, "Cleanup summary: %d removed, %d failed\n", cleanupSucceeded, cleanupFailed)
		}
	}

	if selectErr != nil {
		// Include cleanup context in error when relevant
		if cleanupSucceeded > 0 || cleanupFailed > 0 {
			return fmt.Errorf("%w (cleanup: %d removed, %d failed)", selectErr, cleanupSucceeded, cleanupFailed)
		}
		return selectErr
	}

	// Extract session details for resumption
	var wtState *worktree.WorktreeState
	var st *state.State
	var files []string
	var sessID string
	effectiveWorkingDir := wd

	if selected.Type == session.SessionTypeWorktree {
		wtState = selected.WorktreeState
		files = selected.SpecFiles
		sessID = selected.ID
		effectiveWorkingDir = selected.Path()
		fmt.Printf("Found worktree session: %s (branch: %s)\n", wtState.Path, wtState.Branch)
		fmt.Printf("Resuming in worktree: %s\n\n", effectiveWorkingDir)
	} else {
		// Regular session
		st = selected.RegularState
		files = selected.SpecFiles
		sessID = selected.ID

		// Also check for queued files to merge
		stateDir := state.StateDir(wd)
		queue, err := state.LoadQueue(stateDir)
		if err == nil && !queue.IsEmpty() {
			queuedFiles := queue.Pop()
			files = append(files, queuedFiles...)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Found %d queued file(s)\n", len(queuedFiles))
		}
	}

	// If no files from state or queue, nothing to continue
	if len(files) == 0 {
		return fmt.Errorf("no session to continue in this directory (no active or queued files)")
	}

	// If no existing state, create a new one
	if st == nil {
		sessID = generateSessionID()
		st = state.NewState(sessID, wd, files, "", nil)
		if wtState == nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Starting new session %s with %d file(s)...\n", sessID, len(files))
		}
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

	// Ensure notes directory exists
	notesDir := filepath.Dir(spec.NotesFile)
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
	printSummary(formatter, loopState)

	// Handle state cleanup or preservation
	if err != nil {
		if wtState != nil {
			fmt.Printf("\nWorktree preserved: %s\n", wtState.Path)
			fmt.Println("Run 'orbital continue' to resume.")
		}
		switch err {
		case loop.ErrMaxIterationsReached:
			os.Exit(1)
		case loop.ErrBudgetExceeded:
			os.Exit(2)
		case context.DeadlineExceeded:
			os.Exit(3)
		case context.Canceled:
			fmt.Println("\nInterrupted by user")
			if wtState != nil {
				fmt.Printf("Worktree preserved: %s\n", wtState.Path)
			}
			fmt.Println("Session state preserved. Run 'orbital continue' to resume.")
			os.Exit(130)
		default:
			os.Exit(4)
		}
	}

	// Worktree mode: run merge phase and cleanup
	if wtState != nil {
		fmt.Println("\nWorktree mode: running merge phase...")

		// Pre-merge verification: ensure branches exist
		if err := worktree.VerifyBranchExists(wd, wtState.Branch); err != nil {
			fmt.Fprintf(os.Stderr, "Pre-merge check failed: %v\n", err)
			fmt.Printf("Worktree preserved: %s\n", wtState.Path)
			os.Exit(4)
		}
		if err := worktree.VerifyBranchExists(wd, wtState.OriginalBranch); err != nil {
			fmt.Fprintf(os.Stderr, "Pre-merge check failed: %v\n", err)
			fmt.Printf("Worktree preserved: %s\n", wtState.Path)
			os.Exit(4)
		}

		// Get worktree branch HEAD before merge for verification
		worktreeHead, err := worktree.GetBranchHeadCommit(wd, wtState.Branch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get worktree branch HEAD: %v\n", err)
			fmt.Printf("Worktree preserved: %s\n", wtState.Path)
			os.Exit(4)
		}

		// Run merge phase
		mergeResult, mergeErr := runWorktreeMerge(ctx, worktreeMergeOptions{
			model:          mergeModel,
			worktreePath:   wtState.Path,
			branchName:     wtState.Branch,
			originalBranch: wtState.OriginalBranch,
		})

		if mergeErr != nil {
			fmt.Fprintf(os.Stderr, "Merge phase failed: %v\n", mergeErr)
			fmt.Printf("Worktree preserved: %s\n", wtState.Path)
			fmt.Println("Resolve manually and run 'orbital continue' to retry.")
			os.Exit(4)
		}

		if !mergeResult.Success {
			fmt.Fprintln(os.Stderr, "Merge phase did not complete successfully.")
			fmt.Printf("Worktree preserved: %s\n", wtState.Path)
			fmt.Println("Resolve manually and run 'orbital continue' to retry.")
			os.Exit(4)
		}

		// Post-merge verification: ensure original branch now contains worktree commits
		if err := worktree.VerifyBranchContains(wd, wtState.OriginalBranch, worktreeHead); err != nil {
			fmt.Fprintf(os.Stderr, "Merge verification failed: %v\n", err)
			fmt.Fprintf(os.Stderr, "Claude reported success but git shows the merge did not complete.\n")
			fmt.Printf("Worktree preserved: %s\n", wtState.Path)
			fmt.Println("Resolve manually and run 'orbital continue' to retry.")
			os.Exit(4)
		}

		// Verify we're on the original branch
		if err := worktree.VerifyOnBranch(wd, wtState.OriginalBranch); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			fmt.Println("Merge appears successful but you may not be on the expected branch.")
		}

		fmt.Printf("Merge cost: $%.4f\n", mergeResult.CostUSD)
		fmt.Println("Merge verified: original branch now contains worktree commits.")

		// Cleanup worktree and branch
		cleanup := worktree.NewCleanup(wd)
		if err := cleanup.Run(wtState.Path, wtState.Branch); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to cleanup worktree: %v\n", err)
		} else {
			fmt.Println("Worktree and branch cleaned up.")
		}

		// Remove worktree state entry
		wtStateMgr := worktree.NewStateManager(wd)
		if err := wtStateMgr.Remove(wtState.Path); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove worktree state: %v\n", err)
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

// selectSession handles session selection using the new unified session abstraction.
// Priority:
// 1. --continue-worktree flag specifies exact worktree name
// 2. Single valid session: auto-select
// 3. Multiple sessions: TUI selector (unless --non-interactive)
func selectSession(sessions []session.Session, validator sessionValidator) (*session.Session, []string, error) {
	validSessions := validator.ValidSessions(sessions)

	// Handle --continue-worktree flag (preserved for backwards compatibility)
	if continueWorktree != "" {
		for i := range sessions {
			if sessions[i].Type == session.SessionTypeWorktree && sessions[i].Name == continueWorktree {
				if !sessions[i].Valid {
					return nil, nil, fmt.Errorf("worktree %q is invalid: %s", sessions[i].Name, sessions[i].InvalidReason)
				}
				return &sessions[i], nil, nil
			}
		}
		return nil, nil, fmt.Errorf("worktree not found: %s\nAvailable: %s", continueWorktree, formatSessionNames(sessions))
	}

	// Auto-resume if only one valid session
	if len(validSessions) == 1 {
		return &validSessions[0], nil, nil
	}

	// No valid sessions - show TUI so user can clean up invalid ones
	if len(validSessions) == 0 {
		if nonInteractive {
			return nil, nil, fmt.Errorf("no valid sessions to resume\nUse 'orbital worktree cleanup' to remove stale entries")
		}
	}

	// Multiple sessions or need to show invalid ones - use TUI selector
	if nonInteractive {
		return nil, nil, fmt.Errorf("multiple sessions found; use --continue-worktree to specify:\n%s", formatSessionList(sessions))
	}

	// Run TUI selector
	result, err := selector.Run(sessions)
	if err != nil {
		return nil, nil, fmt.Errorf("session selection failed: %w", err)
	}

	if result.Cancelled {
		// Return cleanup paths even when cancelled so they get processed
		return nil, result.CleanupPaths, fmt.Errorf("selection cancelled")
	}

	if result.Session == nil {
		return nil, result.CleanupPaths, fmt.Errorf("no session selected")
	}

	return result.Session, result.CleanupPaths, nil
}

// formatSessionNames returns a comma-separated list of session names (worktrees only).
func formatSessionNames(sessions []session.Session) string {
	var names []string
	for _, s := range sessions {
		if s.Type == session.SessionTypeWorktree {
			names = append(names, s.Name)
		}
	}
	if len(names) == 0 {
		return "(none)"
	}
	result := names[0]
	for i := 1; i < len(names); i++ {
		result += ", " + names[i]
	}
	return result
}

// formatSessionList formats sessions for display in error messages.
func formatSessionList(sessions []session.Session) string {
	var result string
	for i, s := range sessions {
		status := "valid"
		if !s.Valid {
			status = "invalid: " + s.InvalidReason
		}
		typeLabel := ""
		if s.Type == session.SessionTypeWorktree {
			typeLabel = " (worktree)"
			result += fmt.Sprintf("  [%d] %s%s - %s\n", i+1, s.DisplayName(), typeLabel, status)
			if s.Branch() != "" {
				result += fmt.Sprintf("      Branch: %s\n", s.Branch())
			}
		} else {
			result += fmt.Sprintf("  [%d] %s - %s\n", i+1, s.DisplayName(), status)
		}
	}
	return result
}
