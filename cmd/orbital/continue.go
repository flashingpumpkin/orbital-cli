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
	"github.com/flashingpumpkin/orbital/internal/spec"
	"github.com/flashingpumpkin/orbital/internal/state"
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

	// Check for worktree state first
	var wtState *worktree.WorktreeState
	wtStateManager := worktree.NewStateManager(wd)
	worktrees, err := wtStateManager.List()
	if err == nil && len(worktrees) > 0 {
		// For now, use the first worktree (could add TUI picker later)
		wt := worktrees[0]
		wtState = &wt
		fmt.Printf("Found worktree session: %s (branch: %s)\n", wtState.Path, wtState.Branch)
	}

	var st *state.State
	var files []string
	var sessID string
	effectiveWorkingDir := wd

	// If worktree state exists, use its context
	if wtState != nil {
		files = wtState.SpecFiles
		sessID = wtState.SessionID
		effectiveWorkingDir = wtState.Path
		fmt.Printf("Resuming in worktree: %s\n\n", effectiveWorkingDir)
	} else {
		// Try to load existing state
		if state.Exists(wd) {
			st, err = state.Load(wd)
			if err != nil {
				return fmt.Errorf("failed to load state: %w", err)
			}

			// Check if an instance is already running (PID is alive)
			if !st.IsStale() {
				return fmt.Errorf("orbital instance already running (PID: %d)", st.PID)
			}

			sessID = st.SessionID
			files = st.ActiveFiles
		}

		// Also check for queued files
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
		SpecPath:          files[0],
		MaxIterations:     iterations,
		CompletionPromise: promise,
		Model:             model,
		CheckerModel:      checkerModel,
		MaxBudget:         budget,
		WorkingDir:        effectiveWorkingDir,
		Verbose:           verbose,
		Debug:             debug,
		ShowUnhandled:     showUnhandled,
		DryRun:            dryRun,
		SessionID:         sessionID, // Only if user provided --session-id
		IterationTimeout:  timeout,
		MaxTurns:          maxTurns,
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

		fmt.Printf("Merge cost: $%.4f\n", mergeResult.CostUSD)

		// Cleanup worktree and branch
		cleanup := worktree.NewCleanup(wd)
		if err := cleanup.Run(wtState.Path, wtState.Branch); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to cleanup worktree: %v\n", err)
		} else {
			fmt.Println("Worktree and branch cleaned up.")
		}

		// Remove worktree state entry
		if err := wtStateManager.Remove(wtState.Path); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove worktree state: %v\n", err)
		}
	}

	// On successful completion, clean up state
	if err := cleanupState(st); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to cleanup state: %v\n", err)
	}

	return nil
}
