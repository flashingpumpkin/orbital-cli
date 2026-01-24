package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/flashingpumpkin/orbital/internal/completion"
	"github.com/flashingpumpkin/orbital/internal/config"
	"github.com/flashingpumpkin/orbital/internal/executor"
	"github.com/flashingpumpkin/orbital/internal/loop"
	"github.com/flashingpumpkin/orbital/internal/output"
	"github.com/flashingpumpkin/orbital/internal/spec"
	"github.com/flashingpumpkin/orbital/internal/workflow"
	"github.com/flashingpumpkin/orbital/internal/worktree"
)

// SessionRunner manages session execution.
type SessionRunner struct {
	registry   *Registry
	projectDir string
	config     *DaemonConfig
	mu         sync.Mutex
	cancels    map[string]context.CancelFunc
	mergeLocks map[string]bool // Tracks sessions currently being merged
}

// NewSessionRunner creates a new session runner.
func NewSessionRunner(registry *Registry, projectDir string, cfg *DaemonConfig) *SessionRunner {
	return &SessionRunner{
		registry:   registry,
		projectDir: projectDir,
		config:     cfg,
		cancels:    make(map[string]context.CancelFunc),
		mergeLocks: make(map[string]bool),
	}
}

// Start starts a new session.
func (r *SessionRunner) Start(ctx context.Context, req StartSessionRequest) (*Session, error) {
	// Generate session ID
	sessionID := generateSessionID()

	// Resolve absolute paths for spec files
	absSpecFiles := make([]string, len(req.SpecFiles))
	for i, f := range req.SpecFiles {
		if filepath.IsAbs(f) {
			absSpecFiles[i] = f
		} else {
			absSpecFiles[i] = filepath.Join(r.projectDir, f)
		}
	}

	// Create session
	session := &Session{
		ID:            sessionID,
		SpecFiles:     absSpecFiles,
		Status:        StatusRunning,
		WorkingDir:    r.projectDir,
		Iteration:     0,
		MaxIterations: req.MaxIterations,
		TotalCost:     0,
		MaxBudget:     req.Budget,
		StartedAt:     time.Now(),
		NotesFile:     req.NotesFile,
		ContextFiles:  req.ContextFiles,
		// Store config for resume
		Model:        req.Model,
		CheckerModel: req.CheckerModel,
		WorkflowName: req.Workflow,
		SystemPrompt: req.SystemPrompt,
	}

	// Handle worktree setup if requested
	if req.Worktree {
		wtInfo, err := r.setupWorktree(req.WorktreeName, absSpecFiles)
		if err != nil {
			return nil, fmt.Errorf("worktree setup failed: %w", err)
		}
		session.Worktree = wtInfo
		session.WorkingDir = wtInfo.Path
	}

	// Add to registry
	if err := r.registry.Add(session); err != nil {
		// Cleanup worktree if we created one
		if session.Worktree != nil {
			cleanup := worktree.NewCleanup(r.projectDir)
			cleanup.Run(session.Worktree.Path, session.Worktree.Branch)
		}
		return nil, fmt.Errorf("failed to add session: %w", err)
	}

	// Create cancellable context for the session
	sessionCtx, cancel := context.WithCancel(context.Background())
	r.mu.Lock()
	r.cancels[sessionID] = cancel
	r.mu.Unlock()

	// Start execution in background
	go r.run(sessionCtx, session, req)

	return session, nil
}

// Stop stops a running session.
func (r *SessionRunner) Stop(sessionID string) error {
	r.mu.Lock()
	cancel, exists := r.cancels[sessionID]
	if exists {
		// Delete from map while holding lock to prevent race with run()'s defer
		delete(r.cancels, sessionID)
	}
	r.mu.Unlock()

	if !exists {
		return fmt.Errorf("session %s not found or not running", sessionID)
	}

	// Cancel the context to stop execution (safe to call after delete)
	cancel()

	// Update status
	return r.registry.UpdateStatus(sessionID, StatusStopped, "")
}

// Resume resumes an interrupted or stopped session.
func (r *SessionRunner) Resume(ctx context.Context, sessionID string) error {
	session, exists := r.registry.GetInternal(sessionID)
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	if session.Status != StatusInterrupted && session.Status != StatusStopped {
		return fmt.Errorf("session must be interrupted or stopped to resume")
	}

	// Update status to running
	session.Status = StatusRunning
	if err := r.registry.Update(session); err != nil {
		return err
	}

	// Create new context for resumed session
	sessionCtx, cancel := context.WithCancel(context.Background())
	r.mu.Lock()
	r.cancels[sessionID] = cancel
	r.mu.Unlock()

	// Build request from session state (restore all persisted config)
	req := StartSessionRequest{
		SpecFiles:     session.SpecFiles,
		ContextFiles:  session.ContextFiles,
		NotesFile:     session.NotesFile,
		Budget:        session.MaxBudget,
		MaxIterations: session.MaxIterations,
		Model:         session.Model,
		CheckerModel:  session.CheckerModel,
		Workflow:      session.WorkflowName,
		SystemPrompt:  session.SystemPrompt,
	}

	// Resume execution in background
	go r.run(sessionCtx, session, req)

	return nil
}

// Merge triggers a worktree merge for a completed session.
func (r *SessionRunner) Merge(sessionID string) error {
	// Check and acquire merge lock
	r.mu.Lock()
	if r.mergeLocks[sessionID] {
		r.mu.Unlock()
		return fmt.Errorf("merge already in progress for session %s", sessionID)
	}
	r.mergeLocks[sessionID] = true
	r.mu.Unlock()

	// Ensure we release the lock when done
	defer func() {
		r.mu.Lock()
		delete(r.mergeLocks, sessionID)
		r.mu.Unlock()
	}()

	session, exists := r.registry.GetInternal(sessionID)
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	if session.Worktree == nil {
		return fmt.Errorf("session has no worktree")
	}

	// Update status to merging
	if err := r.registry.UpdateStatus(sessionID, StatusMerging, ""); err != nil {
		return err
	}

	// Broadcast merge start
	r.registry.Broadcast(sessionID, OutputMsg{
		Type:      "status",
		Content:   "Starting merge...",
		Timestamp: time.Now(),
	})

	// Run merge
	mergeCfg := &config.Config{
		Model:     "haiku",
		MaxBudget: 10.0,
	}
	mergeExec := executor.New(mergeCfg)
	adapter := &worktreeExecutorAdapter{exec: mergeExec}

	merge := worktree.NewMerge(adapter)
	mergeOpts := worktree.MergeOptions{
		WorktreePath:   session.Worktree.Path,
		BranchName:     session.Worktree.Branch,
		OriginalBranch: session.Worktree.OriginalBranch,
	}

	result, err := merge.Run(context.Background(), mergeOpts)
	if err != nil {
		r.registry.UpdateStatus(sessionID, StatusFailed, fmt.Sprintf("merge failed: %v", err))
		r.registry.Broadcast(sessionID, OutputMsg{
			Type:      "error",
			Content:   fmt.Sprintf("Merge failed: %v", err),
			Timestamp: time.Now(),
		})
		return err
	}

	if !result.Success {
		r.registry.UpdateStatus(sessionID, StatusConflict, "merge conflict")
		r.registry.Broadcast(sessionID, OutputMsg{
			Type:      "error",
			Content:   "Merge conflict - manual resolution required",
			Timestamp: time.Now(),
		})
		return fmt.Errorf("merge conflict")
	}

	// Cleanup worktree
	cleanup := worktree.NewCleanup(r.projectDir)
	if err := cleanup.Run(session.Worktree.Path, session.Worktree.Branch); err != nil {
		// Log but don't fail
		r.registry.Broadcast(sessionID, OutputMsg{
			Type:      "text",
			Content:   fmt.Sprintf("Warning: worktree cleanup failed: %v", err),
			Timestamp: time.Now(),
		})
	}

	// Update status to merged
	r.registry.UpdateStatus(sessionID, StatusMerged, "")
	r.registry.Broadcast(sessionID, OutputMsg{
		Type:      "status",
		Content:   "Merge completed successfully",
		Timestamp: time.Now(),
	})

	return nil
}

// Chat sends a message to an interactive chat session.
func (r *SessionRunner) Chat(ctx context.Context, session *Session, message string) (string, error) {
	// Get the actual session from registry for updates
	realSession, exists := r.registry.GetInternal(session.ID)
	if !exists {
		return "", fmt.Errorf("session %s not found", session.ID)
	}

	// Create chat executor
	chatCfg := &config.Config{
		Model:      r.config.ChatModel,
		MaxBudget:  r.config.ChatBudget,
		SessionID:  realSession.ChatSession, // Resume if exists
		WorkingDir: realSession.WorkingDir,
	}

	// Build context prompt
	var contextParts []string
	contextParts = append(contextParts, "You are helping iterate on specs for an Orbital session.")
	contextParts = append(contextParts, fmt.Sprintf("The session is working on: %s", strings.Join(realSession.SpecFiles, ", ")))
	if realSession.NotesFile != "" {
		contextParts = append(contextParts, fmt.Sprintf("Notes file: %s", realSession.NotesFile))
	}
	chatCfg.SystemPrompt = strings.Join(contextParts, "\n")

	chatExec := executor.New(chatCfg)
	result, err := chatExec.Execute(ctx, message)
	if err != nil {
		return "", err
	}

	// Parse response to extract text
	parser := output.NewParser()
	var response strings.Builder
	for _, line := range strings.Split(result.Output, "\n") {
		event, _ := parser.ParseLine([]byte(line))
		if event != nil && event.Type == "assistant" {
			response.WriteString(event.Content)
		}
	}

	return response.String(), nil
}

// run executes the main session loop.
func (r *SessionRunner) run(ctx context.Context, session *Session, req StartSessionRequest) {
	defer func() {
		r.mu.Lock()
		// Only delete if still present (Stop() may have already removed it)
		if _, exists := r.cancels[session.ID]; exists {
			delete(r.cancels, session.ID)
		}
		r.mu.Unlock()
	}()

	// Create output writer that broadcasts to subscribers
	outputWriter := &broadcastWriter{
		registry:  r.registry,
		sessionID: session.ID,
	}

	// Build config
	cfg := &config.Config{
		SpecPath:          session.SpecFiles[0],
		MaxIterations:     session.MaxIterations,
		CompletionPromise: "<promise>COMPLETE</promise>",
		Model:             req.Model,
		CheckerModel:      req.CheckerModel,
		MaxBudget:         session.MaxBudget,
		WorkingDir:        session.WorkingDir,
		Verbose:           true,
		SystemPrompt:      req.SystemPrompt,
	}

	// Validate and load spec
	sp, err := spec.Validate(session.SpecFiles)
	if err != nil {
		r.handleError(session.ID, fmt.Errorf("failed to validate spec: %w", err))
		return
	}

	// Build prompt
	prompt := sp.BuildPrompt()

	// Create executor with output streaming
	exec := executor.New(cfg)
	exec.SetStreamWriter(outputWriter)

	// Create completion detector
	detector := completion.New(cfg.CompletionPromise)

	// Create loop controller
	controller := loop.New(cfg, exec, detector)
	controller.SetSpecFiles(session.SpecFiles)

	// Set callbacks
	controller.SetIterationStartCallback(func(iteration, maxIterations int) {
		r.registry.Broadcast(session.ID, OutputMsg{
			Type:      "status",
			Content:   fmt.Sprintf("Starting iteration %d/%d", iteration, maxIterations),
			Timestamp: time.Now(),
		})
	})

	controller.SetIterationCallback(func(iteration int, totalCost float64, tokensIn, tokensOut int) error {
		r.registry.UpdateProgress(session.ID, iteration, totalCost, tokensIn, tokensOut)
		return nil
	})

	// Resolve workflow
	wf, err := r.resolveWorkflow(req.Workflow)
	if err != nil {
		r.handleError(session.ID, fmt.Errorf("failed to resolve workflow: %w", err))
		return
	}

	// Run the loop
	var loopState *loop.LoopState
	if wf.HasGates() {
		loopState, err = r.runWorkflowLoop(ctx, cfg, exec, wf, session, sp)
	} else {
		loopState, err = controller.Run(ctx, prompt)
	}

	// Handle result
	if err != nil {
		switch err {
		case context.Canceled:
			// Already marked as stopped
			return
		case loop.ErrMaxIterationsReached:
			r.registry.UpdateStatus(session.ID, StatusFailed, "max iterations reached")
		case loop.ErrBudgetExceeded:
			r.registry.UpdateStatus(session.ID, StatusFailed, "budget exceeded")
		default:
			r.handleError(session.ID, err)
		}
		return
	}

	if loopState != nil && loopState.Completed {
		r.registry.UpdateStatus(session.ID, StatusCompleted, "")
		r.registry.Broadcast(session.ID, OutputMsg{
			Type:      "status",
			Content:   "Session completed successfully",
			Timestamp: time.Now(),
		})
	}
}

// runWorkflowLoop runs a multi-step workflow.
func (r *SessionRunner) runWorkflowLoop(
	ctx context.Context,
	cfg *config.Config,
	exec *executor.Executor,
	wf *workflow.Workflow,
	session *Session,
	sp *spec.Spec,
) (*loop.LoopState, error) {
	loopState := &loop.LoopState{
		StartTime: time.Now(),
	}

	// Create step executor adapter
	stepExec := &claudeStepExecutor{exec: exec}

	// Create workflow runner
	runner := workflow.NewRunner(wf, stepExec)
	runner.SetFilePaths(session.SpecFiles)

	// Set callbacks
	runner.SetStartCallback(func(info workflow.StepInfo) {
		r.registry.Broadcast(session.ID, OutputMsg{
			Type:      "status",
			Content:   fmt.Sprintf("Step %d/%d: %s", info.Position, info.Total, info.Name),
			Timestamp: time.Now(),
		})
	})

	runner.SetCallback(func(info workflow.StepInfo, result *workflow.ExecutionResult, gateResult workflow.GateResult) error {
		loopState.TotalCost += result.CostUSD
		loopState.TotalTokensIn += result.TokensIn
		loopState.TotalTokensOut += result.TokensOut
		loopState.TotalTokens = loopState.TotalTokensIn + loopState.TotalTokensOut
		loopState.LastOutput = result.Output

		r.registry.UpdateProgress(session.ID, loopState.Iteration, loopState.TotalCost, loopState.TotalTokensIn, loopState.TotalTokensOut)
		return nil
	})

	// Run iterations
	for iteration := 1; iteration <= cfg.MaxIterations; iteration++ {
		loopState.Iteration = iteration

		if ctx.Err() != nil {
			return loopState, ctx.Err()
		}

		runResult, err := runner.Run(ctx)
		if err != nil {
			if err == workflow.ErrMaxGateRetriesExceeded {
				continue
			}
			return loopState, err
		}

		if loopState.TotalCost >= cfg.MaxBudget {
			return loopState, loop.ErrBudgetExceeded
		}

		if runResult.CompletedAllSteps {
			// Run verification
			verifyResult, verifyErr := r.runVerification(ctx, cfg, session.SpecFiles)
			if verifyErr != nil {
				continue
			}

			if verifyResult != nil {
				loopState.TotalCost += verifyResult.Cost
				loopState.TotalTokens += verifyResult.Tokens
			}

			if verifyResult != nil && verifyResult.Verified {
				loopState.Completed = true
				return loopState, nil
			}
		}
	}

	return loopState, loop.ErrMaxIterationsReached
}

// runVerification runs the verification step.
func (r *SessionRunner) runVerification(ctx context.Context, cfg *config.Config, specFiles []string) (*loop.VerificationResult, error) {
	verifyCfg := &config.Config{
		Model:     cfg.CheckerModel,
		MaxBudget: cfg.MaxBudget,
	}

	verifyExec := executor.New(verifyCfg)
	prompt := spec.BuildVerificationPrompt(specFiles)

	result, err := verifyExec.Execute(ctx, prompt)
	if err != nil {
		return nil, err
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

// setupWorktree creates and configures a worktree for the session.
func (r *SessionRunner) setupWorktree(name string, specFiles []string) (*WorktreeInfo, error) {
	if err := worktree.CheckGitRepository(r.projectDir); err != nil {
		return nil, err
	}

	// Generate or use provided name
	if name == "" {
		existing, _ := worktree.ListWorktreeNames(r.projectDir)
		name = worktree.GenerateUniqueName(existing)
	}

	// Get original branch
	originalBranch, err := worktree.GetCurrentBranch(r.projectDir)
	if err != nil {
		return nil, err
	}

	// Create worktree
	if err := worktree.CreateWorktree(r.projectDir, name); err != nil {
		return nil, err
	}

	return &WorktreeInfo{
		Name:           name,
		Path:           worktree.WorktreePath(name),
		Branch:         worktree.BranchName(name),
		OriginalBranch: originalBranch,
	}, nil
}

// resolveWorkflow resolves a workflow by name.
func (r *SessionRunner) resolveWorkflow(name string) (*workflow.Workflow, error) {
	if name == "" {
		name = r.config.DefaultWorkflow
	}

	if !workflow.IsValidPreset(name) {
		return nil, fmt.Errorf("invalid workflow preset: %s", name)
	}

	return workflow.GetPreset(workflow.PresetName(name))
}

// handleError updates session status on error.
func (r *SessionRunner) handleError(sessionID string, err error) {
	r.registry.UpdateStatus(sessionID, StatusFailed, err.Error())
	r.registry.Broadcast(sessionID, OutputMsg{
		Type:      "error",
		Content:   err.Error(),
		Timestamp: time.Now(),
	})
}

// generateSessionID generates a unique session ID.
func generateSessionID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// broadcastWriter is an io.Writer that broadcasts to session subscribers.
type broadcastWriter struct {
	registry  *Registry
	sessionID string
	parser    *output.Parser
}

func (w *broadcastWriter) Write(p []byte) (n int, err error) {
	if w.parser == nil {
		w.parser = output.NewParser()
	}

	// Parse the line
	event, _ := w.parser.ParseLine(p)

	msg := OutputMsg{
		Timestamp: time.Now(),
	}

	if event != nil {
		switch event.Type {
		case "assistant":
			msg.Type = "text"
			msg.Content = event.Content
		case "tool_use":
			msg.Type = "tool"
			msg.Content = fmt.Sprintf("[%s] %s", event.ToolName, event.Content)
		case "result":
			msg.Type = "stats"
			msg.Content = string(p)
		default:
			msg.Type = "text"
			msg.Content = string(p)
		}
	} else {
		msg.Type = "text"
		msg.Content = string(p)
	}

	w.registry.Broadcast(w.sessionID, msg)
	return len(p), nil
}

// worktreeExecutorAdapter adapts executor.Executor to worktree.Executor interface.
type worktreeExecutorAdapter struct {
	exec *executor.Executor
}

func (a *worktreeExecutorAdapter) Execute(ctx context.Context, prompt string) (*worktree.ExecutionResult, error) {
	result, err := a.exec.Execute(ctx, prompt)
	if err != nil {
		return nil, err
	}
	return &worktree.ExecutionResult{
		Output:    result.Output,
		CostUSD:   result.CostUSD,
		TokensIn:  result.TokensIn,
		TokensOut: result.TokensOut,
	}, nil
}

// claudeStepExecutor adapts the executor to workflow.StepExecutor interface.
type claudeStepExecutor struct {
	exec *executor.Executor
}

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

// Ensure broadcastWriter implements io.Writer
var _ io.Writer = (*broadcastWriter)(nil)
