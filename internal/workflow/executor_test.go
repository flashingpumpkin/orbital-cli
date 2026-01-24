package workflow

import (
	"context"
	"errors"
	"testing"
)

// mockStepExecutor is a test mock for StepExecutor.
type mockStepExecutor struct {
	responses map[string]*ExecutionResult
	errors    map[string]error
	calls     []string
	// customHandler allows per-call customisation
	customHandler func(ctx context.Context, stepName string, prompt string) (*ExecutionResult, error)
}

func newMockExecutor() *mockStepExecutor {
	return &mockStepExecutor{
		responses: make(map[string]*ExecutionResult),
		errors:    make(map[string]error),
		calls:     make([]string, 0),
	}
}

func (m *mockStepExecutor) setResponse(stepName string, output string, cost float64, tokens int) {
	m.responses[stepName] = &ExecutionResult{
		StepName:  stepName,
		Output:    output,
		CostUSD:   cost,
		TokensIn:  tokens * 6 / 10,
		TokensOut: tokens * 4 / 10,
	}
}

func (m *mockStepExecutor) setError(stepName string, err error) {
	m.errors[stepName] = err
}

func (m *mockStepExecutor) ExecuteStep(ctx context.Context, stepName string, prompt string) (*ExecutionResult, error) {
	m.calls = append(m.calls, stepName)

	// Use custom handler if set
	if m.customHandler != nil {
		return m.customHandler(ctx, stepName, prompt)
	}

	if err, ok := m.errors[stepName]; ok {
		return nil, err
	}

	if resp, ok := m.responses[stepName]; ok {
		return resp, nil
	}

	// Default response
	return &ExecutionResult{
		StepName:  stepName,
		Output:    "default output",
		CostUSD:   0.01,
		TokensIn:  60,
		TokensOut: 40,
	}, nil
}

func TestRunner_Run_SingleStep(t *testing.T) {
	w := &Workflow{
		Steps: []Step{
			{Name: "implement", Prompt: "Do the thing"},
		},
	}

	exec := newMockExecutor()
	exec.setResponse("implement", "Done!", 0.05, 500)

	runner := NewRunner(w, exec)
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !result.CompletedAllSteps {
		t.Error("CompletedAllSteps = false, want true")
	}

	if len(result.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want 1", len(result.Steps))
	}

	if result.TotalCost != 0.05 {
		t.Errorf("TotalCost = %f, want 0.05", result.TotalCost)
	}

	if len(exec.calls) != 1 || exec.calls[0] != "implement" {
		t.Errorf("calls = %v, want [implement]", exec.calls)
	}
}

func TestRunner_Run_MultipleSteps(t *testing.T) {
	w := &Workflow{
		Steps: []Step{
			{Name: "red", Prompt: "Write test"},
			{Name: "green", Prompt: "Make pass"},
			{Name: "refactor", Prompt: "Clean up"},
		},
	}

	exec := newMockExecutor()
	exec.setResponse("red", "Test written", 0.02, 200)
	exec.setResponse("green", "Test passing", 0.03, 300)
	exec.setResponse("refactor", "Code cleaned", 0.01, 100)

	runner := NewRunner(w, exec)
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !result.CompletedAllSteps {
		t.Error("CompletedAllSteps = false, want true")
	}

	if len(result.Steps) != 3 {
		t.Fatalf("len(Steps) = %d, want 3", len(result.Steps))
	}

	expectedCalls := []string{"red", "green", "refactor"}
	for i, call := range exec.calls {
		if call != expectedCalls[i] {
			t.Errorf("calls[%d] = %q, want %q", i, call, expectedCalls[i])
		}
	}

	// Use approximate comparison for floating point
	expectedCost := 0.06
	if result.TotalCost < expectedCost-0.001 || result.TotalCost > expectedCost+0.001 {
		t.Errorf("TotalCost = %f, want ~%f", result.TotalCost, expectedCost)
	}

	// 200 + 300 + 100 = 600 total, split 60/40
	if result.TotalTokensIn != 360 {
		t.Errorf("TotalTokensIn = %d, want 360", result.TotalTokensIn)
	}
	if result.TotalTokensOut != 240 {
		t.Errorf("TotalTokensOut = %d, want 240", result.TotalTokensOut)
	}
}

func TestRunner_Run_GatePasses(t *testing.T) {
	w := &Workflow{
		Steps: []Step{
			{Name: "implement", Prompt: "Do it"},
			{Name: "review", Prompt: "Review", Gate: true, OnFail: "implement"},
		},
	}

	exec := newMockExecutor()
	exec.setResponse("implement", "Done!", 0.02, 200)
	exec.setResponse("review", "Looks good!\n<gate>PASS</gate>", 0.01, 100)

	runner := NewRunner(w, exec)
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !result.CompletedAllSteps {
		t.Error("CompletedAllSteps = false, want true")
	}

	if len(exec.calls) != 2 {
		t.Errorf("calls = %v, want 2 calls", exec.calls)
	}

	// Check gate result was recorded
	reviewStep := result.Steps[1]
	if reviewStep.GateResult != GatePassed {
		t.Errorf("review GateResult = %v, want GatePassed", reviewStep.GateResult)
	}
}

func TestRunner_Run_GateFailsAndLoopsBack(t *testing.T) {
	w := &Workflow{
		Steps: []Step{
			{Name: "implement", Prompt: "Do it"},
			{Name: "review", Prompt: "Review", Gate: true, OnFail: "implement"},
		},
	}

	exec := newMockExecutor()

	// Track call count to vary responses
	callCount := 0
	exec.customHandler = func(ctx context.Context, stepName string, prompt string) (*ExecutionResult, error) {
		callCount++
		if stepName == "review" {
			if callCount <= 2 {
				// First review fails
				return &ExecutionResult{StepName: "review", Output: "Issues found\n<gate>FAIL</gate>", CostUSD: 0.01, TokensIn: 60, TokensOut: 40}, nil
			}
			// Second review passes
			return &ExecutionResult{StepName: "review", Output: "All good\n<gate>PASS</gate>", CostUSD: 0.01, TokensIn: 60, TokensOut: 40}, nil
		}
		return &ExecutionResult{StepName: stepName, Output: "Done!", CostUSD: 0.02, TokensIn: 120, TokensOut: 80}, nil
	}

	runner := NewRunner(w, exec)
	result, err := runner.Run(context.Background())

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !result.CompletedAllSteps {
		t.Error("CompletedAllSteps = false, want true")
	}

	// Should have: implement -> review (fail) -> implement -> review (pass)
	if callCount != 4 {
		t.Errorf("callCount = %d, want 4", callCount)
	}
}

func TestRunner_Run_MaxGateRetriesExceeded(t *testing.T) {
	w := &Workflow{
		Steps: []Step{
			{Name: "implement", Prompt: "Do it"},
			{Name: "review", Prompt: "Review", Gate: true, OnFail: "implement"},
		},
		MaxGateRetries: 2,
	}

	exec := newMockExecutor()
	exec.setResponse("implement", "Done!", 0.02, 200)
	// Review always fails
	exec.setResponse("review", "Issues found\n<gate>FAIL</gate>", 0.01, 100)

	runner := NewRunner(w, exec)
	_, err := runner.Run(context.Background())

	if err == nil {
		t.Fatal("Run() expected error, got nil")
	}

	if !errors.Is(err, ErrMaxGateRetriesExceeded) {
		t.Errorf("Run() error = %v, want ErrMaxGateRetriesExceeded", err)
	}
}

func TestRunner_Run_StepError(t *testing.T) {
	w := &Workflow{
		Steps: []Step{
			{Name: "implement", Prompt: "Do it"},
		},
	}

	exec := newMockExecutor()
	exec.setError("implement", errors.New("execution failed"))

	runner := NewRunner(w, exec)
	_, err := runner.Run(context.Background())

	if err == nil {
		t.Fatal("Run() expected error, got nil")
	}

	if !errors.Is(err, errors.New("execution failed")) {
		// Just check it contains the step name
		if err.Error() != "step \"implement\" failed: execution failed" {
			t.Errorf("Run() error = %q, want to contain step name", err.Error())
		}
	}
}

func TestRunner_Run_TemplateSubstitution(t *testing.T) {
	w := &Workflow{
		Steps: []Step{
			{Name: "implement", Prompt: "Implement {{files}}"},
		},
	}

	var capturedPrompt string
	exec := newMockExecutor()
	exec.customHandler = func(ctx context.Context, stepName string, prompt string) (*ExecutionResult, error) {
		capturedPrompt = prompt
		return &ExecutionResult{StepName: stepName, Output: "Done", CostUSD: 0.01, TokensIn: 60, TokensOut: 40}, nil
	}

	runner := NewRunner(w, exec)
	runner.SetFilePaths([]string{"/path/to/spec.md", "/path/to/other.md"})

	_, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	expected := "Implement - /path/to/spec.md\n- /path/to/other.md"
	if capturedPrompt != expected {
		t.Errorf("prompt = %q, want %q", capturedPrompt, expected)
	}
}

func TestRunner_Run_Callback(t *testing.T) {
	w := &Workflow{
		Steps: []Step{
			{Name: "step1", Prompt: "First"},
			{Name: "step2", Prompt: "Second"},
		},
	}

	exec := newMockExecutor()

	var callbackInfos []StepInfo
	runner := NewRunner(w, exec)
	runner.SetCallback(func(info StepInfo, result *ExecutionResult, gateResult GateResult) error {
		callbackInfos = append(callbackInfos, info)
		return nil
	})

	_, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(callbackInfos) != 2 {
		t.Errorf("callback calls = %d, want 2", len(callbackInfos))
	}

	// Verify step info is correct
	if callbackInfos[0].Name != "step1" {
		t.Errorf("callbackInfos[0].Name = %q, want %q", callbackInfos[0].Name, "step1")
	}
	if callbackInfos[0].Position != 1 {
		t.Errorf("callbackInfos[0].Position = %d, want 1", callbackInfos[0].Position)
	}
	if callbackInfos[0].Total != 2 {
		t.Errorf("callbackInfos[0].Total = %d, want 2", callbackInfos[0].Total)
	}

	if callbackInfos[1].Name != "step2" {
		t.Errorf("callbackInfos[1].Name = %q, want %q", callbackInfos[1].Name, "step2")
	}
	if callbackInfos[1].Position != 2 {
		t.Errorf("callbackInfos[1].Position = %d, want 2", callbackInfos[1].Position)
	}
}

func TestRunner_Run_CallbackError(t *testing.T) {
	w := &Workflow{
		Steps: []Step{
			{Name: "step1", Prompt: "First"},
		},
	}

	exec := newMockExecutor()

	runner := NewRunner(w, exec)
	runner.SetCallback(func(info StepInfo, result *ExecutionResult, gateResult GateResult) error {
		return errors.New("callback error")
	})

	_, err := runner.Run(context.Background())
	if err == nil {
		t.Fatal("Run() expected error, got nil")
	}

	if err.Error() != "callback error" {
		t.Errorf("Run() error = %q, want \"callback error\"", err.Error())
	}
}

func TestRunner_Run_StartCallback(t *testing.T) {
	w := &Workflow{
		Steps: []Step{
			{Name: "step1", Prompt: "First"},
			{Name: "step2", Prompt: "Second"},
		},
	}

	exec := newMockExecutor()

	var startInfos []StepInfo
	runner := NewRunner(w, exec)
	runner.SetStartCallback(func(info StepInfo) {
		startInfos = append(startInfos, info)
	})

	_, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(startInfos) != 2 {
		t.Errorf("start callback calls = %d, want 2", len(startInfos))
	}

	// Verify step info is correct
	if startInfos[0].Name != "step1" {
		t.Errorf("startInfos[0].Name = %q, want %q", startInfos[0].Name, "step1")
	}
	if startInfos[0].Position != 1 {
		t.Errorf("startInfos[0].Position = %d, want 1", startInfos[0].Position)
	}

	if startInfos[1].Name != "step2" {
		t.Errorf("startInfos[1].Name = %q, want %q", startInfos[1].Name, "step2")
	}
	if startInfos[1].Position != 2 {
		t.Errorf("startInfos[1].Position = %d, want 2", startInfos[1].Position)
	}
}

func TestRunner_Run_CallbackGateRetries(t *testing.T) {
	w := &Workflow{
		Steps: []Step{
			{Name: "implement", Prompt: "Do it"},
			{Name: "review", Prompt: "Review", Gate: true, OnFail: "implement"},
		},
		MaxGateRetries: 3,
	}

	exec := newMockExecutor()

	// Track call count to vary responses
	callCount := 0
	exec.customHandler = func(ctx context.Context, stepName string, prompt string) (*ExecutionResult, error) {
		callCount++
		if stepName == "review" {
			if callCount <= 4 {
				// First two reviews fail
				return &ExecutionResult{StepName: "review", Output: "<gate>FAIL</gate>", CostUSD: 0.01, TokensIn: 60, TokensOut: 40}, nil
			}
			// Third review passes
			return &ExecutionResult{StepName: "review", Output: "<gate>PASS</gate>", CostUSD: 0.01, TokensIn: 60, TokensOut: 40}, nil
		}
		return &ExecutionResult{StepName: stepName, Output: "Done!", CostUSD: 0.02, TokensIn: 120, TokensOut: 80}, nil
	}

	var reviewRetries []int
	runner := NewRunner(w, exec)
	runner.SetCallback(func(info StepInfo, result *ExecutionResult, gateResult GateResult) error {
		if info.Name == "review" {
			reviewRetries = append(reviewRetries, info.GateRetries)
		}
		return nil
	})

	_, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Should see review called 3 times: retries 0, 1, 2
	if len(reviewRetries) != 3 {
		t.Fatalf("review callback calls = %d, want 3", len(reviewRetries))
	}
	if reviewRetries[0] != 0 {
		t.Errorf("reviewRetries[0] = %d, want 0", reviewRetries[0])
	}
	if reviewRetries[1] != 1 {
		t.Errorf("reviewRetries[1] = %d, want 1", reviewRetries[1])
	}
	if reviewRetries[2] != 2 {
		t.Errorf("reviewRetries[2] = %d, want 2", reviewRetries[2])
	}
}
