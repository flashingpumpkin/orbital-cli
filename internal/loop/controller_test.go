package loop

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/flashingpumpkin/orbit-cli/internal/completion"
	"github.com/flashingpumpkin/orbit-cli/internal/config"
	"github.com/flashingpumpkin/orbit-cli/internal/executor"
)

// floatEquals compares two floats for equality within a small epsilon.
func floatEquals(a, b float64) bool {
	return math.Abs(a-b) < 0.0001
}

// mockExecutor is a test double for ExecutorInterface.
type mockExecutor struct {
	results []*executor.ExecutionResult
	errors  []error
	calls   int
	prompts []string
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		results: make([]*executor.ExecutionResult, 0),
		errors:  make([]error, 0),
		prompts: make([]string, 0),
	}
}

func (m *mockExecutor) addResult(result *executor.ExecutionResult, err error) {
	m.results = append(m.results, result)
	m.errors = append(m.errors, err)
}

func (m *mockExecutor) Execute(ctx context.Context, prompt string) (*executor.ExecutionResult, error) {
	m.prompts = append(m.prompts, prompt)
	idx := m.calls
	m.calls++

	if idx >= len(m.results) {
		// Return a default result if no more configured
		return &executor.ExecutionResult{
			Output:     "default output",
			Completed:  true,
			TokensUsed: 100,
			CostUSD:    0.01,
		}, nil
	}

	return m.results[idx], m.errors[idx]
}

func TestNew(t *testing.T) {
	cfg := config.NewConfig()
	exec := newMockExecutor()
	det := completion.New("<promise>COMPLETE</promise>")

	ctrl := New(cfg, exec, det)

	if ctrl == nil {
		t.Fatal("expected controller to be created")
	}
	if ctrl.config != cfg {
		t.Error("expected config to be set")
	}
	if ctrl.executor != exec {
		t.Error("expected executor to be set")
	}
	if ctrl.detector != det {
		t.Error("expected detector to be set")
	}
}

func TestRun_CompletesOnFirstIteration(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxIterations = 10
	cfg.MaxBudget = 100.0

	exec := newMockExecutor()
	exec.addResult(&executor.ExecutionResult{
		Output:     "Task done! <promise>COMPLETE</promise>",
		Completed:  true,
		TokensUsed: 500,
		CostUSD:    0.05,
	}, nil)

	det := completion.New("<promise>COMPLETE</promise>")
	ctrl := New(cfg, exec, det)
	ctrl.SetVerifier(newMockVerifier())

	ctx := context.Background()
	state, err := ctrl.Run(ctx, "test prompt")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !state.Completed {
		t.Error("expected Completed to be true")
	}
	if state.Iteration != 1 {
		t.Errorf("expected Iteration to be 1, got %d", state.Iteration)
	}
	// TotalCost includes verification cost (0.05 + 0.001)
	if !floatEquals(state.TotalCost, 0.051) {
		t.Errorf("expected TotalCost to be 0.051, got %f", state.TotalCost)
	}
	// TotalTokens includes verification tokens (500 + 50)
	if state.TotalTokens != 550 {
		t.Errorf("expected TotalTokens to be 550, got %d", state.TotalTokens)
	}
	if state.LastOutput != "Task done! <promise>COMPLETE</promise>" {
		t.Errorf("expected LastOutput to match, got: %s", state.LastOutput)
	}
	if exec.calls != 1 {
		t.Errorf("expected 1 executor call, got %d", exec.calls)
	}
}

func TestRun_CompletesOnThirdIteration(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxIterations = 10
	cfg.MaxBudget = 100.0

	exec := newMockExecutor()
	// First two iterations without completion
	exec.addResult(&executor.ExecutionResult{
		Output:     "Working on it...",
		Completed:  true,
		TokensUsed: 100,
		CostUSD:    0.01,
	}, nil)
	exec.addResult(&executor.ExecutionResult{
		Output:     "Still working...",
		Completed:  true,
		TokensUsed: 150,
		CostUSD:    0.02,
	}, nil)
	// Third iteration with completion
	exec.addResult(&executor.ExecutionResult{
		Output:     "Done! <promise>COMPLETE</promise>",
		Completed:  true,
		TokensUsed: 200,
		CostUSD:    0.03,
	}, nil)

	det := completion.New("<promise>COMPLETE</promise>")
	ctrl := New(cfg, exec, det)
	ctrl.SetVerifier(newMockVerifier())

	ctx := context.Background()
	state, err := ctrl.Run(ctx, "test prompt")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !state.Completed {
		t.Error("expected Completed to be true")
	}
	if state.Iteration != 3 {
		t.Errorf("expected Iteration to be 3, got %d", state.Iteration)
	}
	// TotalCost includes verification cost (0.06 + 0.001)
	if !floatEquals(state.TotalCost, 0.061) {
		t.Errorf("expected TotalCost to be 0.061, got %f", state.TotalCost)
	}
	// TotalTokens includes verification tokens (450 + 50)
	if state.TotalTokens != 500 {
		t.Errorf("expected TotalTokens to be 500, got %d", state.TotalTokens)
	}
	if exec.calls != 3 {
		t.Errorf("expected 3 executor calls, got %d", exec.calls)
	}
}

func TestRun_BudgetExceeded(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxIterations = 10
	cfg.MaxBudget = 0.05 // Low budget

	exec := newMockExecutor()
	// Each iteration costs 0.03
	exec.addResult(&executor.ExecutionResult{
		Output:     "Working...",
		Completed:  true,
		TokensUsed: 100,
		CostUSD:    0.03,
	}, nil)
	exec.addResult(&executor.ExecutionResult{
		Output:     "Still working...",
		Completed:  true,
		TokensUsed: 100,
		CostUSD:    0.03,
	}, nil)

	det := completion.New("<promise>COMPLETE</promise>")
	ctrl := New(cfg, exec, det)

	ctx := context.Background()
	state, err := ctrl.Run(ctx, "test prompt")

	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("expected ErrBudgetExceeded, got: %v", err)
	}
	if state.Completed {
		t.Error("expected Completed to be false")
	}
	if !errors.Is(state.Error, ErrBudgetExceeded) {
		t.Errorf("expected state.Error to be ErrBudgetExceeded, got: %v", state.Error)
	}
	if state.TotalCost != 0.06 {
		t.Errorf("expected TotalCost to be 0.06, got %f", state.TotalCost)
	}
	if state.Iteration != 2 {
		t.Errorf("expected Iteration to be 2, got %d", state.Iteration)
	}
}

func TestRun_MaxIterationsReached(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxIterations = 3
	cfg.MaxBudget = 100.0

	exec := newMockExecutor()
	// None of the iterations complete
	for i := 0; i < 3; i++ {
		exec.addResult(&executor.ExecutionResult{
			Output:     "Working...",
			Completed:  true,
			TokensUsed: 100,
			CostUSD:    0.01,
		}, nil)
	}

	det := completion.New("<promise>COMPLETE</promise>")
	ctrl := New(cfg, exec, det)

	ctx := context.Background()
	state, err := ctrl.Run(ctx, "test prompt")

	if !errors.Is(err, ErrMaxIterationsReached) {
		t.Fatalf("expected ErrMaxIterationsReached, got: %v", err)
	}
	if state.Completed {
		t.Error("expected Completed to be false")
	}
	if !errors.Is(state.Error, ErrMaxIterationsReached) {
		t.Errorf("expected state.Error to be ErrMaxIterationsReached, got: %v", state.Error)
	}
	if state.Iteration != 3 {
		t.Errorf("expected Iteration to be 3, got %d", state.Iteration)
	}
	if exec.calls != 3 {
		t.Errorf("expected 3 executor calls, got %d", exec.calls)
	}
}

func TestRun_ExecutorError(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxIterations = 10
	cfg.MaxBudget = 100.0

	exec := newMockExecutor()
	execErr := errors.New("executor failed")
	exec.addResult(nil, execErr)

	det := completion.New("<promise>COMPLETE</promise>")
	ctrl := New(cfg, exec, det)

	ctx := context.Background()
	state, err := ctrl.Run(ctx, "test prompt")

	if err != execErr {
		t.Fatalf("expected executor error, got: %v", err)
	}
	if state.Completed {
		t.Error("expected Completed to be false")
	}
	if state.Error != execErr {
		t.Errorf("expected state.Error to match, got: %v", state.Error)
	}
	if state.Iteration != 1 {
		t.Errorf("expected Iteration to be 1, got %d", state.Iteration)
	}
}

func TestRun_ContextCancelled(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxIterations = 10
	cfg.MaxBudget = 100.0

	exec := newMockExecutor()
	det := completion.New("<promise>COMPLETE</promise>")
	ctrl := New(cfg, exec, det)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	state, err := ctrl.Run(ctx, "test prompt")

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	if state.Completed {
		t.Error("expected Completed to be false")
	}
	if !errors.Is(state.Error, context.Canceled) {
		t.Errorf("expected state.Error to be context.Canceled, got: %v", state.Error)
	}
	if exec.calls != 0 {
		t.Errorf("expected 0 executor calls, got %d", exec.calls)
	}
}

func TestRun_PromptsPassedCorrectly(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxIterations = 3
	cfg.MaxBudget = 100.0

	exec := newMockExecutor()
	exec.addResult(&executor.ExecutionResult{
		Output:     "<promise>COMPLETE</promise>",
		Completed:  true,
		TokensUsed: 100,
		CostUSD:    0.01,
	}, nil)

	det := completion.New("<promise>COMPLETE</promise>")
	ctrl := New(cfg, exec, det)
	ctrl.SetVerifier(newMockVerifier())

	ctx := context.Background()
	expectedPrompt := "my special prompt"
	_, _ = ctrl.Run(ctx, expectedPrompt)

	if len(exec.prompts) != 1 {
		t.Fatalf("expected 1 prompt recorded, got %d", len(exec.prompts))
	}
	if exec.prompts[0] != expectedPrompt {
		t.Errorf("expected prompt to be %q, got %q", expectedPrompt, exec.prompts[0])
	}
}

func TestRun_StartTimeSet(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxIterations = 1
	cfg.MaxBudget = 100.0

	exec := newMockExecutor()
	exec.addResult(&executor.ExecutionResult{
		Output:     "<promise>COMPLETE</promise>",
		Completed:  true,
		TokensUsed: 100,
		CostUSD:    0.01,
	}, nil)

	det := completion.New("<promise>COMPLETE</promise>")
	ctrl := New(cfg, exec, det)
	ctrl.SetVerifier(newMockVerifier())

	before := time.Now()
	ctx := context.Background()
	state, _ := ctrl.Run(ctx, "prompt")
	after := time.Now()

	if state.StartTime.Before(before) || state.StartTime.After(after) {
		t.Errorf("StartTime should be between %v and %v, got %v", before, after, state.StartTime)
	}
}

func TestRun_BudgetExactlyMet(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxIterations = 10
	cfg.MaxBudget = 0.05

	exec := newMockExecutor()
	// Cost exactly equals budget
	exec.addResult(&executor.ExecutionResult{
		Output:     "Working...",
		Completed:  true,
		TokensUsed: 100,
		CostUSD:    0.05,
	}, nil)

	det := completion.New("<promise>COMPLETE</promise>")
	ctrl := New(cfg, exec, det)

	ctx := context.Background()
	state, err := ctrl.Run(ctx, "test prompt")

	// Budget is exactly met, should trigger exceeded
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("expected ErrBudgetExceeded when cost equals budget, got: %v", err)
	}
	if state.Completed {
		t.Error("expected Completed to be false")
	}
}

func TestRun_ZeroIterations(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxIterations = 0
	cfg.MaxBudget = 100.0

	exec := newMockExecutor()
	det := completion.New("<promise>COMPLETE</promise>")
	ctrl := New(cfg, exec, det)

	ctx := context.Background()
	state, err := ctrl.Run(ctx, "test prompt")

	if !errors.Is(err, ErrMaxIterationsReached) {
		t.Fatalf("expected ErrMaxIterationsReached, got: %v", err)
	}
	if state.Iteration != 0 {
		t.Errorf("expected Iteration to be 0, got %d", state.Iteration)
	}
	if exec.calls != 0 {
		t.Errorf("expected 0 executor calls, got %d", exec.calls)
	}
}

func TestRun_CumulativeTokenTracking(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxIterations = 5
	cfg.MaxBudget = 100.0

	exec := newMockExecutor()
	exec.addResult(&executor.ExecutionResult{
		Output:     "Step 1",
		TokensUsed: 100,
		CostUSD:    0.01,
	}, nil)
	exec.addResult(&executor.ExecutionResult{
		Output:     "Step 2",
		TokensUsed: 200,
		CostUSD:    0.02,
	}, nil)
	exec.addResult(&executor.ExecutionResult{
		Output:     "Step 3",
		TokensUsed: 300,
		CostUSD:    0.03,
	}, nil)
	exec.addResult(&executor.ExecutionResult{
		Output:     "Done! <promise>COMPLETE</promise>",
		TokensUsed: 400,
		CostUSD:    0.04,
	}, nil)

	det := completion.New("<promise>COMPLETE</promise>")
	ctrl := New(cfg, exec, det)
	ctrl.SetVerifier(newMockVerifier())

	ctx := context.Background()
	state, err := ctrl.Run(ctx, "prompt")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	// TotalTokens includes verification tokens (1000 + 50)
	if state.TotalTokens != 1050 {
		t.Errorf("expected TotalTokens to be 1050, got %d", state.TotalTokens)
	}
	// TotalCost includes verification cost (0.10 + 0.001)
	if !floatEquals(state.TotalCost, 0.101) {
		t.Errorf("expected TotalCost to be 0.101, got %f", state.TotalCost)
	}
}

func TestLoopState_InitialValues(t *testing.T) {
	state := LoopState{}

	if state.Iteration != 0 {
		t.Errorf("expected Iteration to be 0, got %d", state.Iteration)
	}
	if state.TotalCost != 0 {
		t.Errorf("expected TotalCost to be 0, got %f", state.TotalCost)
	}
	if state.TotalTokens != 0 {
		t.Errorf("expected TotalTokens to be 0, got %d", state.TotalTokens)
	}
	if state.Completed {
		t.Error("expected Completed to be false")
	}
	if state.Error != nil {
		t.Error("expected Error to be nil")
	}
	if state.LastOutput != "" {
		t.Errorf("expected LastOutput to be empty, got %q", state.LastOutput)
	}
}

// mockStateManager is a test double for StateManager interface.
type mockStateManager struct {
	queuedFiles   []string
	mergedFiles   []string
	rebuildPrompt string
	popCalled     bool
	mergeCalled   bool
	rebuildCalled bool
	popError      error
	mergeError    error
	rebuildError  error
}

func newMockStateManager() *mockStateManager {
	return &mockStateManager{
		queuedFiles:   []string{},
		rebuildPrompt: "rebuilt prompt",
	}
}

func (m *mockStateManager) CheckQueue() ([]string, error) {
	return m.queuedFiles, nil
}

func (m *mockStateManager) PopQueue() ([]string, error) {
	m.popCalled = true
	if m.popError != nil {
		return nil, m.popError
	}
	files := m.queuedFiles
	m.queuedFiles = []string{} // Clear queue after pop
	return files, nil
}

func (m *mockStateManager) MergeFiles(files []string) error {
	m.mergeCalled = true
	if m.mergeError != nil {
		return m.mergeError
	}
	m.mergedFiles = append(m.mergedFiles, files...)
	return nil
}

func (m *mockStateManager) RebuildPrompt() (string, error) {
	m.rebuildCalled = true
	if m.rebuildError != nil {
		return "", m.rebuildError
	}
	return m.rebuildPrompt, nil
}

// mockVerifier is a test double for Verifier interface.
type mockVerifier struct {
	result *VerificationResult
	err    error
	calls  int
}

func newMockVerifier() *mockVerifier {
	return &mockVerifier{
		result: &VerificationResult{
			Verified:  true,
			Unchecked: 0,
			Checked:   5,
			Cost:      0.001,
			Tokens:    50,
		},
	}
}

func (m *mockVerifier) Verify(ctx context.Context, files []string) (*VerificationResult, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestRun_ContinuesWithQueuedFiles(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxIterations = 10
	cfg.MaxBudget = 100.0

	exec := newMockExecutor()
	// First iteration completes
	exec.addResult(&executor.ExecutionResult{
		Output:     "Done! <promise>COMPLETE</promise>",
		Completed:  true,
		TokensUsed: 100,
		CostUSD:    0.01,
	}, nil)
	// Second iteration with new files, also completes
	exec.addResult(&executor.ExecutionResult{
		Output:     "Done with queued files! <promise>COMPLETE</promise>",
		Completed:  true,
		TokensUsed: 100,
		CostUSD:    0.01,
	}, nil)

	det := completion.New("<promise>COMPLETE</promise>")
	ctrl := New(cfg, exec, det)
	ctrl.SetVerifier(newMockVerifier())

	// Set up state manager with queued files
	sm := newMockStateManager()
	sm.queuedFiles = []string{"/path/to/spec2.md"}
	sm.rebuildPrompt = "new prompt with spec2"
	ctrl.SetStateManager(sm)

	ctx := context.Background()
	state, err := ctrl.Run(ctx, "initial prompt")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !state.Completed {
		t.Error("expected Completed to be true")
	}
	if state.Iteration != 2 {
		t.Errorf("expected Iteration to be 2, got %d", state.Iteration)
	}
	if exec.calls != 2 {
		t.Errorf("expected 2 executor calls, got %d", exec.calls)
	}
	if !sm.popCalled {
		t.Error("expected PopQueue to be called")
	}
	if !sm.mergeCalled {
		t.Error("expected MergeFiles to be called")
	}
	if !sm.rebuildCalled {
		t.Error("expected RebuildPrompt to be called")
	}
	if len(sm.mergedFiles) != 1 || sm.mergedFiles[0] != "/path/to/spec2.md" {
		t.Errorf("expected merged files to be [/path/to/spec2.md], got %v", sm.mergedFiles)
	}
	// Check that second call used the rebuilt prompt
	if len(exec.prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(exec.prompts))
	}
	if exec.prompts[0] != "initial prompt" {
		t.Errorf("expected first prompt to be 'initial prompt', got %q", exec.prompts[0])
	}
	if exec.prompts[1] != "new prompt with spec2" {
		t.Errorf("expected second prompt to be 'new prompt with spec2', got %q", exec.prompts[1])
	}
}

func TestRun_ExitsWhenQueueEmpty(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxIterations = 10
	cfg.MaxBudget = 100.0

	exec := newMockExecutor()
	exec.addResult(&executor.ExecutionResult{
		Output:     "Done! <promise>COMPLETE</promise>",
		Completed:  true,
		TokensUsed: 100,
		CostUSD:    0.01,
	}, nil)

	det := completion.New("<promise>COMPLETE</promise>")
	ctrl := New(cfg, exec, det)
	ctrl.SetVerifier(newMockVerifier())

	// Set up state manager with empty queue
	sm := newMockStateManager()
	sm.queuedFiles = []string{} // Empty queue
	ctrl.SetStateManager(sm)

	ctx := context.Background()
	state, err := ctrl.Run(ctx, "prompt")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !state.Completed {
		t.Error("expected Completed to be true")
	}
	if state.Iteration != 1 {
		t.Errorf("expected Iteration to be 1, got %d", state.Iteration)
	}
	if exec.calls != 1 {
		t.Errorf("expected 1 executor call, got %d", exec.calls)
	}
	if !sm.popCalled {
		t.Error("expected PopQueue to be called")
	}
	// MergeFiles and RebuildPrompt should NOT be called when queue is empty
	if sm.mergeCalled {
		t.Error("expected MergeFiles NOT to be called when queue empty")
	}
	if sm.rebuildCalled {
		t.Error("expected RebuildPrompt NOT to be called when queue empty")
	}
}

func TestRun_BackwardCompatibleWithoutStateManager(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxIterations = 10
	cfg.MaxBudget = 100.0

	exec := newMockExecutor()
	exec.addResult(&executor.ExecutionResult{
		Output:     "Done! <promise>COMPLETE</promise>",
		Completed:  true,
		TokensUsed: 100,
		CostUSD:    0.01,
	}, nil)

	det := completion.New("<promise>COMPLETE</promise>")
	ctrl := New(cfg, exec, det)
	ctrl.SetVerifier(newMockVerifier())
	// No StateManager set - should work exactly as before

	ctx := context.Background()
	state, err := ctrl.Run(ctx, "prompt")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !state.Completed {
		t.Error("expected Completed to be true")
	}
	if state.Iteration != 1 {
		t.Errorf("expected Iteration to be 1, got %d", state.Iteration)
	}
}

func TestRun_PopQueueError(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxIterations = 10
	cfg.MaxBudget = 100.0

	exec := newMockExecutor()
	exec.addResult(&executor.ExecutionResult{
		Output:     "Done! <promise>COMPLETE</promise>",
		Completed:  true,
		TokensUsed: 100,
		CostUSD:    0.01,
	}, nil)

	det := completion.New("<promise>COMPLETE</promise>")
	ctrl := New(cfg, exec, det)
	ctrl.SetVerifier(newMockVerifier())

	sm := newMockStateManager()
	sm.popError = errors.New("pop queue failed")
	ctrl.SetStateManager(sm)

	ctx := context.Background()
	state, err := ctrl.Run(ctx, "prompt")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "pop queue failed" {
		t.Errorf("expected 'pop queue failed' error, got: %v", err)
	}
	if state.Completed {
		t.Error("expected Completed to be false")
	}
	if state.Error == nil || state.Error.Error() != "pop queue failed" {
		t.Errorf("expected state.Error to be 'pop queue failed', got: %v", state.Error)
	}
}

func TestRun_MergeFilesError(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxIterations = 10
	cfg.MaxBudget = 100.0

	exec := newMockExecutor()
	exec.addResult(&executor.ExecutionResult{
		Output:     "Done! <promise>COMPLETE</promise>",
		Completed:  true,
		TokensUsed: 100,
		CostUSD:    0.01,
	}, nil)

	det := completion.New("<promise>COMPLETE</promise>")
	ctrl := New(cfg, exec, det)
	ctrl.SetVerifier(newMockVerifier())

	sm := newMockStateManager()
	sm.queuedFiles = []string{"/path/to/spec.md"}
	sm.mergeError = errors.New("merge failed")
	ctrl.SetStateManager(sm)

	ctx := context.Background()
	state, err := ctrl.Run(ctx, "prompt")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "merge failed" {
		t.Errorf("expected 'merge failed' error, got: %v", err)
	}
	if state.Completed {
		t.Error("expected Completed to be false")
	}
}

func TestRun_RebuildPromptError(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxIterations = 10
	cfg.MaxBudget = 100.0

	exec := newMockExecutor()
	exec.addResult(&executor.ExecutionResult{
		Output:     "Done! <promise>COMPLETE</promise>",
		Completed:  true,
		TokensUsed: 100,
		CostUSD:    0.01,
	}, nil)

	det := completion.New("<promise>COMPLETE</promise>")
	ctrl := New(cfg, exec, det)
	ctrl.SetVerifier(newMockVerifier())

	sm := newMockStateManager()
	sm.queuedFiles = []string{"/path/to/spec.md"}
	sm.rebuildError = errors.New("rebuild failed")
	ctrl.SetStateManager(sm)

	ctx := context.Background()
	state, err := ctrl.Run(ctx, "prompt")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "rebuild failed" {
		t.Errorf("expected 'rebuild failed' error, got: %v", err)
	}
	if state.Completed {
		t.Error("expected Completed to be false")
	}
}

func TestSetStateManager(t *testing.T) {
	cfg := config.NewConfig()
	exec := newMockExecutor()
	det := completion.New("<promise>COMPLETE</promise>")

	ctrl := New(cfg, exec, det)

	if ctrl.stateManager != nil {
		t.Error("expected stateManager to be nil initially")
	}

	sm := newMockStateManager()
	ctrl.SetStateManager(sm)

	if ctrl.stateManager != sm {
		t.Error("expected stateManager to be set")
	}
}

func TestSetSpecFiles(t *testing.T) {
	cfg := config.NewConfig()
	exec := newMockExecutor()
	det := completion.New("<promise>COMPLETE</promise>")

	ctrl := New(cfg, exec, det)

	if len(ctrl.specFiles) != 0 {
		t.Error("expected specFiles to be empty initially")
	}

	files := []string{"/path/to/spec1.md", "/path/to/spec2.md"}
	ctrl.SetSpecFiles(files)

	if len(ctrl.specFiles) != 2 {
		t.Errorf("expected 2 spec files, got %d", len(ctrl.specFiles))
	}
	if ctrl.specFiles[0] != "/path/to/spec1.md" {
		t.Errorf("expected first file to be '/path/to/spec1.md', got %q", ctrl.specFiles[0])
	}
}

func TestParseVerificationResponse_Verified(t *testing.T) {
	output := "VERIFIED: 0 unchecked, 5 checked"

	verified, unchecked, checked := ParseVerificationResponse(output)

	if !verified {
		t.Error("expected verified to be true")
	}
	if unchecked != 0 {
		t.Errorf("expected unchecked to be 0, got %d", unchecked)
	}
	if checked != 5 {
		t.Errorf("expected checked to be 5, got %d", checked)
	}
}

func TestParseVerificationResponse_Incomplete(t *testing.T) {
	output := "INCOMPLETE: 3 unchecked, 7 checked"

	verified, unchecked, checked := ParseVerificationResponse(output)

	if verified {
		t.Error("expected verified to be false")
	}
	if unchecked != 3 {
		t.Errorf("expected unchecked to be 3, got %d", unchecked)
	}
	if checked != 7 {
		t.Errorf("expected checked to be 7, got %d", checked)
	}
}

func TestParseVerificationResponse_UnparseableOutput(t *testing.T) {
	output := "I don't understand the question"

	verified, unchecked, checked := ParseVerificationResponse(output)

	if verified {
		t.Error("expected verified to be false for unparseable output")
	}
	if unchecked != -1 {
		t.Errorf("expected unchecked to be -1, got %d", unchecked)
	}
	if checked != -1 {
		t.Errorf("expected checked to be -1, got %d", checked)
	}
}

func TestParseVerificationResponse_WithExtraText(t *testing.T) {
	output := `I've checked the spec files.

Here are the results:
VERIFIED: 0 unchecked, 10 checked

All items are complete!`

	verified, unchecked, checked := ParseVerificationResponse(output)

	if !verified {
		t.Error("expected verified to be true")
	}
	if unchecked != 0 {
		t.Errorf("expected unchecked to be 0, got %d", unchecked)
	}
	if checked != 10 {
		t.Errorf("expected checked to be 10, got %d", checked)
	}
}

func TestParseVerificationResponse_IncompleteWithExtraText(t *testing.T) {
	output := `Checking spec files...

INCOMPLETE: 2 unchecked, 8 checked

You still have work to do.`

	verified, unchecked, checked := ParseVerificationResponse(output)

	if verified {
		t.Error("expected verified to be false")
	}
	if unchecked != 2 {
		t.Errorf("expected unchecked to be 2, got %d", unchecked)
	}
	if checked != 8 {
		t.Errorf("expected checked to be 8, got %d", checked)
	}
}

func TestRun_VerificationFailureContinuesLoop(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxIterations = 3
	cfg.MaxBudget = 100.0

	exec := newMockExecutor()
	// First iteration: outputs promise but verification fails
	exec.addResult(&executor.ExecutionResult{
		Output:     "Done! <promise>COMPLETE</promise>",
		Completed:  true,
		TokensUsed: 100,
		CostUSD:    0.01,
	}, nil)
	// Second iteration: outputs promise, verification passes
	exec.addResult(&executor.ExecutionResult{
		Output:     "Really done! <promise>COMPLETE</promise>",
		Completed:  true,
		TokensUsed: 100,
		CostUSD:    0.01,
	}, nil)

	det := completion.New("<promise>COMPLETE</promise>")
	ctrl := New(cfg, exec, det)

	// Mock verifier that fails first time, succeeds second time
	verifier := &sequenceVerifier{
		results: []*VerificationResult{
			{Verified: false, Unchecked: 2, Checked: 3, Cost: 0.001, Tokens: 50},
			{Verified: true, Unchecked: 0, Checked: 5, Cost: 0.001, Tokens: 50},
		},
	}
	ctrl.SetVerifier(verifier)

	ctx := context.Background()
	state, err := ctrl.Run(ctx, "prompt")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !state.Completed {
		t.Error("expected Completed to be true")
	}
	if state.Iteration != 2 {
		t.Errorf("expected Iteration to be 2 (first verification failed), got %d", state.Iteration)
	}
	if exec.calls != 2 {
		t.Errorf("expected 2 executor calls, got %d", exec.calls)
	}
	if verifier.calls != 2 {
		t.Errorf("expected 2 verifier calls, got %d", verifier.calls)
	}
}

func TestRun_VerificationErrorContinuesLoop(t *testing.T) {
	cfg := config.NewConfig()
	cfg.MaxIterations = 3
	cfg.MaxBudget = 100.0

	exec := newMockExecutor()
	// First iteration: outputs promise but verification errors
	exec.addResult(&executor.ExecutionResult{
		Output:     "Done! <promise>COMPLETE</promise>",
		Completed:  true,
		TokensUsed: 100,
		CostUSD:    0.01,
	}, nil)
	// Second iteration: outputs promise, verification passes
	exec.addResult(&executor.ExecutionResult{
		Output:     "Really done! <promise>COMPLETE</promise>",
		Completed:  true,
		TokensUsed: 100,
		CostUSD:    0.01,
	}, nil)

	det := completion.New("<promise>COMPLETE</promise>")
	ctrl := New(cfg, exec, det)

	// Mock verifier that errors first time, succeeds second time
	verifier := &errorThenSuccessVerifier{
		err:    errors.New("verification failed"),
		result: &VerificationResult{Verified: true, Unchecked: 0, Checked: 5, Cost: 0.001, Tokens: 50},
	}
	ctrl.SetVerifier(verifier)

	ctx := context.Background()
	state, err := ctrl.Run(ctx, "prompt")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !state.Completed {
		t.Error("expected Completed to be true")
	}
	if state.Iteration != 2 {
		t.Errorf("expected Iteration to be 2 (first verification errored), got %d", state.Iteration)
	}
}

// sequenceVerifier returns results in sequence for testing.
type sequenceVerifier struct {
	results []*VerificationResult
	calls   int
}

func (v *sequenceVerifier) Verify(ctx context.Context, files []string) (*VerificationResult, error) {
	idx := v.calls
	v.calls++
	if idx < len(v.results) {
		return v.results[idx], nil
	}
	// Default to verified if we run out of configured results
	return &VerificationResult{Verified: true, Unchecked: 0, Checked: 5}, nil
}

// errorThenSuccessVerifier returns an error first, then succeeds.
type errorThenSuccessVerifier struct {
	err    error
	result *VerificationResult
	calls  int
}

func (v *errorThenSuccessVerifier) Verify(ctx context.Context, files []string) (*VerificationResult, error) {
	v.calls++
	if v.calls == 1 {
		return nil, v.err
	}
	return v.result, nil
}
