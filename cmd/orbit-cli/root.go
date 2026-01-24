// Package main provides the CLI entry point for orbit.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/flashingpumpkin/orbit-cli/internal/completion"
	"github.com/flashingpumpkin/orbit-cli/internal/config"
	"github.com/flashingpumpkin/orbit-cli/internal/executor"
	"github.com/flashingpumpkin/orbit-cli/internal/loop"
	"github.com/flashingpumpkin/orbit-cli/internal/output"
	"github.com/flashingpumpkin/orbit-cli/internal/spec"
	"github.com/flashingpumpkin/orbit-cli/internal/state"
	"github.com/flashingpumpkin/orbit-cli/internal/tui"
	"github.com/flashingpumpkin/orbit-cli/internal/workflow"
	"golang.org/x/term"
)

var (
	// Flag variables
	iterations    int
	promise       string
	model         string
	checkerModel  string
	budget        float64
	workingDir    string
	configFile    string
	quiet         bool
	debug         bool
	showUnhandled bool
	todosOnly     bool
	dryRun        bool
	sessionID     string
	timeout       time.Duration
	maxTurns      int
	systemPrompt  string
	agents        string
	notesFile     string
	contextFiles  []string
	workflowFlag  string
	minimal       bool
	worktree      bool
)

var rootCmd = &cobra.Command{
	Use:     "orbit-cli <spec-file>",
	Short:   "Autonomous Claude Code iteration loop",
	Long: `Orbit implements the "Ralph Wiggum" method for autonomous Claude Code execution.

It runs Claude Code in a loop, monitoring output for a completion promise string.
The loop continues until the promise is detected, max iterations reached, or
budget is exhausted.

Named after Ralph Wiggum's optimistic persistence: "I'm learnding!"

USAGE

    orbit-cli <spec-file> [--context <file>]... [--notes <file>] [flags]

The spec file contains the main task specification. Additional context files
can be provided with --context (repeatable). A notes file for cross-iteration
context can be specified with --notes.

CONFIGURATION FILE

Orbit can be configured via a TOML file. By default, it looks for .orbit/config.toml
in the working directory. Use --config to specify a different path.`,
	Args:    cobra.ExactArgs(1),
	Version: "0.1.0",
	RunE:    runOrbit,
}

func init() {
	// Register subcommands
	rootCmd.AddCommand(continueCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(statusCmd)

	// Register persistent flags (inherited by subcommands like 'continue')
	rootCmd.PersistentFlags().IntVarP(&iterations, "iterations", "n", 50, "Maximum number of loop iterations")
	rootCmd.PersistentFlags().StringVarP(&promise, "promise", "p", "<promise>COMPLETE</promise>", "Completion promise string to detect")
	rootCmd.PersistentFlags().StringVarP(&model, "model", "m", "opus", "Claude model to use for execution")
	rootCmd.PersistentFlags().StringVar(&checkerModel, "checker-model", "haiku", "Claude model to use for completion checking")
	rootCmd.PersistentFlags().Float64VarP(&budget, "budget", "b", 100.00, "Maximum budget in USD")
	rootCmd.PersistentFlags().StringVarP(&workingDir, "working-dir", "d", ".", "Working directory for execution")
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Path to config file (default: .orbit/config.toml)")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress verbose output")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Stream all raw JSON output from Claude")
	rootCmd.PersistentFlags().BoolVar(&showUnhandled, "show-unhandled", false, "Show raw JSON for unhandled event types")
	rootCmd.PersistentFlags().BoolVar(&todosOnly, "todos-only", false, "Only show TodoWrite output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Run without executing commands")
	rootCmd.PersistentFlags().StringVarP(&sessionID, "session-id", "s", "", "Session ID for resuming")
	rootCmd.PersistentFlags().DurationVarP(&timeout, "timeout", "t", 30*time.Minute, "Timeout per iteration")
	rootCmd.PersistentFlags().IntVar(&maxTurns, "max-turns", 0, "Max agentic turns per iteration (0 = unlimited)")
	rootCmd.PersistentFlags().StringVar(&systemPrompt, "system-prompt", "", "Custom system prompt (overrides default)")
	rootCmd.PersistentFlags().StringVar(&agents, "agents", "", "JSON object defining custom agents for Claude CLI")
	rootCmd.PersistentFlags().StringVar(&notesFile, "notes", "", "Path to notes file (default: auto-generated in docs/notes/)")
	rootCmd.PersistentFlags().StringArrayVar(&contextFiles, "context", []string{}, "Additional context file (can be repeated)")
	rootCmd.PersistentFlags().StringVar(&workflowFlag, "workflow", "", "Workflow preset: fast, spec-driven (default), reviewed, tdd")
	rootCmd.PersistentFlags().BoolVar(&minimal, "minimal", false, "Use minimal output mode (no TUI)")
	rootCmd.PersistentFlags().BoolVar(&worktree, "worktree", false, "Enable worktree isolation mode")
}

func runOrbit(cmd *cobra.Command, args []string) error {
	specPath := args[0]

	// Build list of all files: spec file + context files
	allFiles := append([]string{specPath}, contextFiles...)

	// Get absolute paths for all files
	absFilePaths, err := getAbsolutePaths(allFiles)
	if err != nil {
		return err
	}

	// Verbose is default, quiet suppresses it
	verbose := !quiet

	// Create config from flags
	// Note: SessionID is only set if explicitly provided via --session-id flag
	// for resuming an existing Claude session. For new sessions, leave it empty.
	cfg := &config.Config{
		SpecPath:          specPath,
		MaxIterations:     iterations,
		CompletionPromise: promise,
		Model:             model,
		CheckerModel:      checkerModel,
		MaxBudget:         budget,
		WorkingDir:        workingDir,
		Verbose:           verbose,
		Debug:             debug,
		ShowUnhandled:     showUnhandled,
		DryRun:            dryRun,
		SessionID:         sessionID, // Only use if explicitly provided
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
		// Use explicit config file path
		fileConfig, err = config.LoadFileConfigFrom(configFile)
		if err != nil {
			return fmt.Errorf("failed to load config file %s: %w", configFile, err)
		}
		if fileConfig == nil {
			return fmt.Errorf("config file not found: %s", configFile)
		}
	} else {
		// Try default .orbit/config.toml
		fileConfig, err = config.LoadFileConfig(workingDir)
		if err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
	}
	if fileConfig != nil && fileConfig.Prompt != "" {
		spec.PromptTemplate = fileConfig.Prompt
	}

	// Handle agents: CLI flag takes precedence over config file
	if agents != "" {
		if err := config.ValidateAgentsJSON(agents); err != nil {
			return fmt.Errorf("invalid --agents flag: %w", err)
		}
		cfg.Agents = agents
	} else if fileConfig != nil && len(fileConfig.Agents) > 0 {
		agentsJSON, err := config.AgentsToJSON(fileConfig.Agents)
		if err != nil {
			return fmt.Errorf("failed to convert agents config: %w", err)
		}
		cfg.Agents = agentsJSON
	}

	// Set completion promise for prompt template
	spec.CompletionPromise = cfg.CompletionPromise

	// Set notes file path (from flag or auto-generate)
	if notesFile != "" {
		spec.NotesFile = notesFile
	} else {
		spec.NotesFile = generateNotesFilePath(specPath)
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

	// Validate spec and context files exist
	sp, err := spec.Validate(allFiles)
	if err != nil {
		return fmt.Errorf("failed to validate files: %w", err)
	}

	// Create completion detector
	detector := completion.New(cfg.CompletionPromise)

	// Create executor
	exec := executor.New(cfg)

	// Determine if we should use TUI mode
	useTUI := shouldUseTUI()

	// TUI program (only set if useTUI is true)
	var tuiProgram *tui.Program

	// Enable streaming output
	if cfg.Debug {
		// Debug mode: stream raw JSON (no TUI)
		exec.SetStreamWriter(os.Stdout)
	} else if useTUI {
		// TUI mode: create program and bridge
		session := tui.SessionInfo{
			SpecFiles:   absFilePaths,
			NotesFile:   spec.NotesFile,
			StateFile:   state.StateDir(workingDir) + "/state.json",
			ContextFile: strings.Join(contextFiles, ", "),
		}
		progress := tui.ProgressInfo{
			Iteration:    1,
			MaxIteration: cfg.MaxIterations,
			Budget:       cfg.MaxBudget,
		}
		tuiProgram = tui.New(session, progress)
		exec.SetStreamWriter(tuiProgram.Bridge())
	} else if cfg.Verbose || cfg.ShowUnhandled || todosOnly {
		// Minimal/verbose mode: formatted output
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

	// Set spec file paths for verification
	controller.SetSpecFiles(absFilePaths)

	// Generate a state ID for orbit's internal tracking (separate from Claude session ID)
	stateID := generateSessionID()

	// Initialize session state
	st, err := initState(stateID, workingDir, absFilePaths, spec.NotesFile, contextFiles)
	if err != nil {
		return fmt.Errorf("failed to initialize state: %w", err)
	}

	// Set up state manager for queue checking after completion
	sm, err := newStateManagerAdapter(st, sp)
	if err != nil {
		return fmt.Errorf("failed to create state manager: %w", err)
	}
	controller.SetStateManager(sm)

	// Set iteration callback to update state after each iteration
	controller.SetIterationCallback(func(iteration int, totalCost float64, totalTokensIn, totalTokensOut int) error {
		// Update state
		if err := updateState(st, iteration, totalCost); err != nil {
			return err
		}

		// Send progress update to TUI if active
		if tuiProgram != nil {
			tuiProgram.SendProgress(tui.ProgressInfo{
				Iteration:    iteration,
				MaxIteration: cfg.MaxIterations,
				TokensIn:     totalTokensIn,
				TokensOut:    totalTokensOut,
				Cost:         totalCost,
				Budget:       cfg.MaxBudget,
			})
		}

		return nil
	})

	// Resolve workflow from flag or config
	wf, err := resolveWorkflow(workflowFlag, fileConfig)
	if err != nil {
		return fmt.Errorf("failed to resolve workflow: %w", err)
	}

	// Build the prompt
	prompt := sp.BuildPrompt()

	// Print banner or start TUI
	if !useTUI {
		printBanner(cfg, sp, contextFiles, wf)

		// Print the command that will be executed
		if cfg.Verbose {
			fmt.Println("Command:")
			fmt.Printf("  %s\n", exec.GetCommand(prompt))
			fmt.Println()
		}
	}

	// Create context with signal handling for graceful shutdown
	ctx, cancel := setupSignalHandler()
	defer cancel()

	// Run the loop (with TUI in background if enabled)
	var loopState *loop.LoopState
	if tuiProgram != nil {
		// Run TUI in a goroutine
		tuiDone := make(chan error, 1)
		go func() {
			tuiDone <- tuiProgram.Run()
		}()

		// Check if workflow has gates (multi-step workflow)
		if wf.HasGates() {
			loopState, err = runWorkflowLoop(ctx, cfg, exec, wf, absFilePaths, sm, st, tuiProgram)
		} else {
			loopState, err = controller.Run(ctx, prompt)
		}

		// Quit the TUI
		tuiProgram.Quit()
		<-tuiDone
	} else {
		// Check if workflow has gates (multi-step workflow)
		if wf.HasGates() {
			loopState, err = runWorkflowLoop(ctx, cfg, exec, wf, absFilePaths, sm, st, nil)
		} else {
			loopState, err = controller.Run(ctx, prompt)
		}
	}

	// Print summary (only in minimal mode)
	if !useTUI {
		printSummary(loopState)
	}

	// Handle state cleanup or preservation
	if err != nil {
		// On error or interrupt, preserve state for resume
		// State is already saved by iteration callback, so no action needed
		switch err {
		case loop.ErrMaxIterationsReached:
			os.Exit(1)
		case loop.ErrBudgetExceeded:
			os.Exit(2)
		case context.DeadlineExceeded:
			os.Exit(3)
		case context.Canceled:
			fmt.Println("\nInterrupted by user")
			fmt.Println("Session state preserved. Run 'orbit-cli continue' to resume.")
			os.Exit(130)
		default:
			os.Exit(4)
		}
	}

	// On successful completion, clean up state
	if err := cleanupState(st); err != nil {
		// Log but don't fail - the work is done
		fmt.Fprintf(os.Stderr, "Warning: failed to cleanup state: %v\n", err)
	}

	return nil
}

func printBanner(cfg *config.Config, sp *spec.Spec, ctxFiles []string, wf *workflow.Workflow) {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    Orbit - I'm learnding!                     ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  Spec:        %s\n", sp.FilePaths[0])
	if len(ctxFiles) > 0 {
		fmt.Printf("  Context:     %d file(s)\n", len(ctxFiles))
		for _, path := range ctxFiles {
			fmt.Printf("               - %s\n", path)
		}
	}
	fmt.Printf("  Workflow:    %s", wf.Name)
	if wf.HasGates() {
		fmt.Printf(" (%d steps, with gates)\n", len(wf.Steps))
	} else {
		fmt.Printf(" (%d step)\n", len(wf.Steps))
	}
	fmt.Printf("  Model:       %s\n", cfg.Model)
	fmt.Printf("  Checker:     %s\n", cfg.CheckerModel)
	fmt.Printf("  Iterations:  max %d\n", cfg.MaxIterations)
	fmt.Printf("  Budget:      $%.2f USD\n", cfg.MaxBudget)
	fmt.Printf("  Timeout:     %v per iteration\n", cfg.IterationTimeout)
	fmt.Printf("  Working Dir: %s\n", cfg.WorkingDir)
	fmt.Printf("  Notes File:  %s\n", spec.NotesFile)
	if cfg.SessionID != "" {
		fmt.Printf("  Resuming:    session %s\n", cfg.SessionID)
	}
	if cfg.DryRun {
		fmt.Println("  Mode:        DRY RUN (no commands will be executed)")
	}
	if cfg.Debug {
		fmt.Println("  Debug:       enabled (raw JSON output)")
	}
	fmt.Println()
	fmt.Println("Starting loop...")
	fmt.Println()
}

func printSummary(loopState *loop.LoopState) {
	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════════════")
	fmt.Println("                           Summary                              ")
	fmt.Println("════════════════════════════════════════════════════════════════")
	fmt.Printf("  Iterations:  %d\n", loopState.Iteration)
	fmt.Printf("  Total Cost:  $%.4f USD\n", loopState.TotalCost)
	fmt.Printf("  Total Tokens: %d\n", loopState.TotalTokens)
	fmt.Printf("  Duration:    %v\n", time.Since(loopState.StartTime).Round(time.Second))

	if loopState.Completed {
		fmt.Println("  Status:      COMPLETED (promise detected)")
	} else if loopState.Error != nil {
		fmt.Printf("  Status:      TERMINATED (%v)\n", loopState.Error)
	}
	fmt.Println()
}

// generateSessionID generates a unique session ID.
func generateSessionID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		panic(fmt.Sprintf("failed to generate session ID: crypto/rand.Read failed: %v", err))
	}
	return hex.EncodeToString(bytes)
}

// initState creates and saves a new session state.
func initState(sessionID, workingDir string, files []string, notesFile string, contextFiles []string) (*state.State, error) {
	st := state.NewState(sessionID, workingDir, files, notesFile, contextFiles)
	if err := st.Save(); err != nil {
		return nil, fmt.Errorf("failed to save state: %w", err)
	}
	return st, nil
}

// updateState updates the iteration count and total cost in the state.
func updateState(st *state.State, iteration int, totalCost float64) error {
	st.UpdateIteration(iteration, totalCost)
	return st.Save()
}

// cleanupState removes the state directory.
func cleanupState(st *state.State) error {
	return st.Cleanup()
}

// getAbsolutePaths converts relative paths to absolute paths.
func getAbsolutePaths(paths []string) ([]string, error) {
	result := make([]string, len(paths))
	for i, p := range paths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for %s: %w", p, err)
		}
		result[i] = absPath
	}
	return result, nil
}

// stateManagerAdapter implements loop.StateManager interface.
type stateManagerAdapter struct {
	st    *state.State
	sp    *spec.Spec
	queue *state.Queue
}

// newStateManagerAdapter creates a new StateManager adapter.
func newStateManagerAdapter(st *state.State, sp *spec.Spec) (*stateManagerAdapter, error) {
	stateDir := state.StateDir(st.WorkingDir)
	queue, err := state.LoadQueue(stateDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load queue: %w", err)
	}
	return &stateManagerAdapter{st: st, sp: sp, queue: queue}, nil
}

// CheckQueue returns any queued files without removing them.
func (m *stateManagerAdapter) CheckQueue() ([]string, error) {
	// Reload queue to get latest state
	stateDir := state.StateDir(m.st.WorkingDir)
	queue, err := state.LoadQueue(stateDir)
	if err != nil {
		return nil, err
	}
	m.queue = queue
	return queue.QueuedFiles, nil
}

// PopQueue returns and removes all queued files.
func (m *stateManagerAdapter) PopQueue() ([]string, error) {
	// Reload queue to get latest state
	stateDir := state.StateDir(m.st.WorkingDir)
	queue, err := state.LoadQueue(stateDir)
	if err != nil {
		return nil, err
	}
	m.queue = queue
	files := queue.Pop()
	return files, nil
}

// MergeFiles adds files to the active file list and updates state.
func (m *stateManagerAdapter) MergeFiles(files []string) error {
	// Add to spec's file paths
	m.sp.FilePaths = append(m.sp.FilePaths, files...)

	// Update state's active files
	m.st.ActiveFiles = append(m.st.ActiveFiles, files...)

	// Save updated state
	return m.st.Save()
}

// RebuildPrompt rebuilds the prompt with the current active files.
func (m *stateManagerAdapter) RebuildPrompt() (string, error) {
	return m.sp.BuildPrompt(), nil
}

// generateNotesFilePath generates the notes file path from the spec file.
// Format: docs/notes/<YYYY-MM-DD>-notes-<feature-slug>.md
func generateNotesFilePath(specPath string) string {
	// Extract base name without extension
	base := filepath.Base(specPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	// Strip any leading date prefix (YYYY-MM-DD-) to avoid duplication
	datePrefix := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-`)
	name = datePrefix.ReplaceAllString(name, "")

	// Convert to kebab-case slug
	slug := toKebabCase(name)

	// Generate date prefix
	date := time.Now().Format("2006-01-02")

	return filepath.Join("docs", "notes", fmt.Sprintf("%s-notes-%s.md", date, slug))
}

// toKebabCase converts a string to kebab-case.
func toKebabCase(s string) string {
	// Replace underscores and spaces with hyphens
	s = strings.ReplaceAll(s, "_", "-")
	s = strings.ReplaceAll(s, " ", "-")

	// Insert hyphens before uppercase letters and convert to lowercase
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('-')
		}
		result.WriteRune(r)
	}

	// Convert to lowercase and clean up multiple hyphens
	kebab := strings.ToLower(result.String())
	multiHyphen := regexp.MustCompile(`-+`)
	kebab = multiHyphen.ReplaceAllString(kebab, "-")
	kebab = strings.Trim(kebab, "-")

	return kebab
}

// shouldUseTUI determines whether to use the TUI based on flags and environment.
func shouldUseTUI() bool {
	// Explicit minimal flag disables TUI
	if minimal {
		return false
	}

	// Debug mode disables TUI (raw JSON output)
	if debug {
		return false
	}

	// Quiet mode disables TUI
	if quiet {
		return false
	}

	// CI environment disables TUI
	if os.Getenv("CI") != "" {
		return false
	}

	// Non-interactive terminal disables TUI
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return false
	}

	return true
}

// resolveWorkflow determines the workflow to use based on CLI flag and config file.
// CLI flag takes precedence over config file. If neither specified, uses spec-driven default.
func resolveWorkflow(flagValue string, fileConfig *config.FileConfig) (*workflow.Workflow, error) {
	// CLI flag takes precedence
	if flagValue != "" {
		if !workflow.IsValidPreset(flagValue) {
			validPresets := workflow.ValidPresets()
			names := make([]string, len(validPresets))
			for i, p := range validPresets {
				names[i] = string(p)
			}
			return nil, fmt.Errorf("invalid workflow preset %q, valid options: %s", flagValue, strings.Join(names, ", "))
		}
		return workflow.GetPreset(workflow.PresetName(flagValue))
	}

	// Check config file
	if fileConfig != nil && fileConfig.Workflow != nil {
		return fileConfig.Workflow.ToWorkflow()
	}

	// Default to spec-driven
	return workflow.GetPreset(workflow.PresetSpecDriven)
}

// claudeStepExecutor adapts the executor.Executor to the workflow.StepExecutor interface.
type claudeStepExecutor struct {
	exec *executor.Executor
}

// ExecuteStep executes a single workflow step by invoking Claude with the step prompt.
func (e *claudeStepExecutor) ExecuteStep(ctx context.Context, stepName string, prompt string) (*workflow.ExecutionResult, error) {
	result, err := e.exec.Execute(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("step %q execution failed: %w", stepName, err)
	}

	return &workflow.ExecutionResult{
		StepName:  stepName,
		Output:    result.Output,
		CostUSD:   result.CostUSD,
		TokensIn:  result.TokensIn,
		TokensOut: result.TokensOut,
	}, nil
}

// runWorkflowLoop executes a multi-step workflow with gates.
// It runs the workflow steps in sequence, handling gate pass/fail logic,
// and iterates until verification passes or limits are reached.
func runWorkflowLoop(
	ctx context.Context,
	cfg *config.Config,
	exec *executor.Executor,
	wf *workflow.Workflow,
	specFiles []string,
	sm *stateManagerAdapter,
	st *state.State,
	tuiProgram *tui.Program,
) (*loop.LoopState, error) {
	loopState := &loop.LoopState{
		StartTime: time.Now(),
	}

	// Create step executor adapter
	stepExec := &claudeStepExecutor{exec: exec}

	// Create workflow runner
	runner := workflow.NewRunner(wf, stepExec)
	runner.SetFilePaths(specFiles)

	// Set callback to track step execution
	runner.SetCallback(func(stepName string, result *workflow.ExecutionResult, gateResult workflow.GateResult) error {
		// Update totals
		loopState.TotalCost += result.CostUSD
		loopState.TotalTokensIn += result.TokensIn
		loopState.TotalTokensOut += result.TokensOut
		loopState.TotalTokens = loopState.TotalTokensIn + loopState.TotalTokensOut
		loopState.LastOutput = result.Output

		// Send progress update to TUI if active
		if tuiProgram != nil {
			tuiProgram.SendProgress(tui.ProgressInfo{
				Iteration:    loopState.Iteration,
				MaxIteration: cfg.MaxIterations,
				TokensIn:     loopState.TotalTokensIn,
				TokensOut:    loopState.TotalTokensOut,
				Cost:         loopState.TotalCost,
				Budget:       cfg.MaxBudget,
			})
		}

		// Log step completion (non-TUI mode)
		if tuiProgram == nil {
			if gateResult == workflow.GatePassed || gateResult == workflow.GateFailed {
				fmt.Printf("\n[%s] Step completed: gate %s\n", stepName, gateResult)
			} else {
				fmt.Printf("\n[%s] Step completed\n", stepName)
			}
		}

		return nil
	})

	// Outer loop: iterate until verification passes or limits reached
	for iteration := 1; iteration <= cfg.MaxIterations; iteration++ {
		loopState.Iteration = iteration

		// Check context cancellation
		if ctx.Err() != nil {
			loopState.Error = ctx.Err()
			return loopState, ctx.Err()
		}

		if tuiProgram == nil {
			fmt.Printf("\n══════════════════════════════════════════════════════════════\n")
			fmt.Printf("  Iteration %d - Workflow: %s\n", iteration, wf.Name)
			fmt.Printf("══════════════════════════════════════════════════════════════\n\n")
		}

		// Run the workflow (all steps)
		runResult, err := runner.Run(ctx)

		// Update iteration callback
		if err := updateState(st, iteration, loopState.TotalCost); err != nil {
			loopState.Error = err
			return loopState, err
		}

		if err != nil {
			// Check for max gate retries exceeded
			if errors.Is(err, workflow.ErrMaxGateRetriesExceeded) {
				if tuiProgram == nil {
					fmt.Printf("\nWorkflow gate failed too many times: %v\n", err)
				}
				// Continue to next iteration rather than failing completely
				continue
			}
			loopState.Error = err
			return loopState, err
		}

		// Check budget
		if loopState.TotalCost >= cfg.MaxBudget {
			loopState.Error = loop.ErrBudgetExceeded
			return loopState, loop.ErrBudgetExceeded
		}

		// Workflow completed all steps (including gates passing)
		if runResult.CompletedAllSteps {
			if tuiProgram == nil {
				fmt.Println("\nWorkflow completed. Running verification...")
			}

			// Run verification
			verifyResult, verifyErr := runVerification(ctx, cfg, specFiles)

			// Add verification cost
			if verifyResult != nil {
				loopState.TotalCost += verifyResult.Cost
				loopState.TotalTokens += verifyResult.Tokens
			}

			if verifyErr != nil {
				if tuiProgram == nil {
					fmt.Printf("Verification error: %v. Continuing.\n", verifyErr)
				}
				continue
			}

			if !verifyResult.Verified {
				if tuiProgram == nil {
					if verifyResult.Unchecked >= 0 {
						fmt.Printf("Verification: %d unchecked item(s) remain. Continuing.\n", verifyResult.Unchecked)
					} else {
						fmt.Println("Verification: could not parse response. Continuing.")
					}
				}
				continue
			}

			// Verification passed
			if tuiProgram == nil {
				fmt.Printf("Verification: all items complete (%d checked).\n", verifyResult.Checked)
			}

			// Check queue for new files
			if sm != nil {
				queuedFiles, err := sm.PopQueue()
				if err != nil {
					loopState.Error = err
					return loopState, err
				}

				if len(queuedFiles) > 0 {
					if tuiProgram == nil {
						fmt.Printf("Found %d queued file(s), continuing...\n", len(queuedFiles))
						for _, f := range queuedFiles {
							fmt.Printf("  + %s\n", f)
						}
					}

					if err := sm.MergeFiles(queuedFiles); err != nil {
						loopState.Error = err
						return loopState, err
					}

					// Update runner's file paths
					runner.SetFilePaths(append(specFiles, queuedFiles...))
					continue
				}
			}

			// Done
			if tuiProgram == nil {
				fmt.Println("No queued files. Work complete.")
			}
			loopState.Completed = true
			return loopState, nil
		}
	}

	// Max iterations reached
	loopState.Error = loop.ErrMaxIterationsReached
	return loopState, loop.ErrMaxIterationsReached
}

// runVerification executes verification using the checker model.
func runVerification(ctx context.Context, cfg *config.Config, specFiles []string) (*loop.VerificationResult, error) {
	verifyConfig := &config.Config{
		Model:     cfg.CheckerModel,
		MaxBudget: cfg.MaxBudget,
	}

	verifyExec := executor.New(verifyConfig)
	prompt := spec.BuildVerificationPrompt(specFiles)

	result, err := verifyExec.Execute(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("verification execution failed: %w", err)
	}

	verified, unchecked, checked := loop.ParseVerificationResponse(result.Output)

	return &loop.VerificationResult{
		Verified:  verified,
		Unchecked: unchecked,
		Checked:   checked,
		Cost:      result.CostUSD,
		Tokens:    result.TokensIn + result.TokensOut,
	}, nil
}
