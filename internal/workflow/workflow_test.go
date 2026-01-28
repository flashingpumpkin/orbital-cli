package workflow

import (
	"testing"
	"time"
)

func TestWorkflow_Validate(t *testing.T) {
	tests := []struct {
		name     string
		workflow Workflow
		wantErr  string
	}{
		{
			name: "valid single step",
			workflow: Workflow{
				Steps: []Step{
					{Name: "implement", Prompt: "Do the thing"},
				},
			},
			wantErr: "",
		},
		{
			name: "valid multi-step",
			workflow: Workflow{
				Steps: []Step{
					{Name: "implement", Prompt: "Implement the feature"},
					{Name: "review", Prompt: "Review the changes", Gate: true, OnFail: "implement"},
				},
			},
			wantErr: "",
		},
		{
			name: "valid preset only",
			workflow: Workflow{
				Preset: "tdd",
			},
			wantErr: "",
		},
		{
			name:     "empty workflow",
			workflow: Workflow{},
			wantErr:  "workflow must have at least one step or specify a preset",
		},
		{
			name: "missing step name",
			workflow: Workflow{
				Steps: []Step{
					{Prompt: "Do the thing"},
				},
			},
			wantErr: "step 1: name is required",
		},
		{
			name: "missing step prompt",
			workflow: Workflow{
				Steps: []Step{
					{Name: "implement"},
				},
			},
			wantErr: "step 1 (implement): prompt is required",
		},
		{
			name: "duplicate step name",
			workflow: Workflow{
				Steps: []Step{
					{Name: "implement", Prompt: "First"},
					{Name: "implement", Prompt: "Second"},
				},
			},
			wantErr: "step 2: duplicate step name \"implement\"",
		},
		{
			name: "on_fail without gate",
			workflow: Workflow{
				Steps: []Step{
					{Name: "implement", Prompt: "Do it", OnFail: "implement"},
				},
			},
			wantErr: "step 1 (implement): on_fail requires gate = true",
		},
		{
			name: "on_fail references unknown step",
			workflow: Workflow{
				Steps: []Step{
					{Name: "review", Prompt: "Review", Gate: true, OnFail: "nonexistent"},
				},
			},
			wantErr: "step 1 (review): on_fail references unknown step \"nonexistent\"",
		},
		{
			name: "valid deferred step targeted by on_fail",
			workflow: Workflow{
				Steps: []Step{
					{Name: "implement", Prompt: "Do it"},
					{Name: "fix", Prompt: "Fix it", Deferred: true},
					{Name: "review", Prompt: "Review", Gate: true, OnFail: "fix"},
				},
			},
			wantErr: "",
		},
		{
			name: "deferred step not targeted by any on_fail",
			workflow: Workflow{
				Steps: []Step{
					{Name: "implement", Prompt: "Do it"},
					{Name: "fix", Prompt: "Fix it", Deferred: true},
					{Name: "review", Prompt: "Review", Gate: true, OnFail: "implement"},
				},
			},
			wantErr: "step 2 (fix): deferred step is unreachable (not targeted by any on_fail)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.workflow.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.wantErr)
				} else if err.Error() != tt.wantErr {
					t.Errorf("Validate() error = %q, want %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestWorkflow_GetStepIndex(t *testing.T) {
	w := Workflow{
		Steps: []Step{
			{Name: "red", Prompt: "Write test"},
			{Name: "green", Prompt: "Make pass"},
			{Name: "refactor", Prompt: "Clean up"},
		},
	}

	tests := []struct {
		name      string
		stepName  string
		wantIndex int
	}{
		{"first step", "red", 0},
		{"middle step", "green", 1},
		{"last step", "refactor", 2},
		{"not found", "nonexistent", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := w.GetStepIndex(tt.stepName)
			if got != tt.wantIndex {
				t.Errorf("GetStepIndex(%q) = %d, want %d", tt.stepName, got, tt.wantIndex)
			}
		})
	}
}

func TestWorkflow_EffectiveMaxGateRetries(t *testing.T) {
	tests := []struct {
		name     string
		workflow Workflow
		want     int
	}{
		{
			name:     "default",
			workflow: Workflow{},
			want:     DefaultMaxGateRetries,
		},
		{
			name:     "custom",
			workflow: Workflow{MaxGateRetries: 5},
			want:     5,
		},
		{
			name:     "zero uses default",
			workflow: Workflow{MaxGateRetries: 0},
			want:     DefaultMaxGateRetries,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.workflow.EffectiveMaxGateRetries()
			if got != tt.want {
				t.Errorf("EffectiveMaxGateRetries() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestDuration_UnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"seconds", "30s", 30 * time.Second, false},
		{"minutes", "5m", 5 * time.Minute, false},
		{"hours", "2h", 2 * time.Hour, false},
		{"combined", "1h30m", 90 * time.Minute, false},
		{"milliseconds", "500ms", 500 * time.Millisecond, false},
		{"complex", "1h2m3s", time.Hour + 2*time.Minute + 3*time.Second, false},
		{"invalid", "invalid", 0, true},
		{"empty", "", 0, true},
		{"number only", "10", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Duration
			err := d.UnmarshalText([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Errorf("UnmarshalText(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("UnmarshalText(%q) unexpected error: %v", tt.input, err)
				return
			}
			if time.Duration(d) != tt.want {
				t.Errorf("UnmarshalText(%q) = %v, want %v", tt.input, time.Duration(d), tt.want)
			}
		})
	}
}

func TestDuration_Duration(t *testing.T) {
	d := Duration(5 * time.Minute)
	if got := d.Duration(); got != 5*time.Minute {
		t.Errorf("Duration() = %v, want %v", got, 5*time.Minute)
	}
}

func TestStep_EffectiveTimeout(t *testing.T) {
	tests := []struct {
		name string
		step Step
		want time.Duration
	}{
		{
			name: "default timeout",
			step: Step{Name: "test", Prompt: "test"},
			want: DefaultStepTimeout,
		},
		{
			name: "custom timeout",
			step: Step{Name: "test", Prompt: "test", Timeout: Duration(10 * time.Minute)},
			want: 10 * time.Minute,
		},
		{
			name: "zero uses default",
			step: Step{Name: "test", Prompt: "test", Timeout: 0},
			want: DefaultStepTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.step.EffectiveTimeout()
			if got != tt.want {
				t.Errorf("EffectiveTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkflow_SetAllStepTimeouts(t *testing.T) {
	w := Workflow{
		Steps: []Step{
			{Name: "step1", Prompt: "prompt1"},
			{Name: "step2", Prompt: "prompt2", Timeout: Duration(10 * time.Minute)},
			{Name: "step3", Prompt: "prompt3"},
		},
	}

	w.SetAllStepTimeouts(15 * time.Minute)

	for i, step := range w.Steps {
		if step.EffectiveTimeout() != 15*time.Minute {
			t.Errorf("Step %d timeout = %v, want %v", i, step.EffectiveTimeout(), 15*time.Minute)
		}
	}
}
