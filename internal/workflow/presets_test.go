package workflow

import (
	"strings"
	"testing"
)

func TestIsValidPreset(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"fast", true},
		{"spec-driven", true},
		{"reviewed", true},
		{"tdd", true},
		{"autonomous", true},
		{"invalid", false},
		{"", false},
		{"TDD", false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidPreset(tt.name)
			if got != tt.want {
				t.Errorf("IsValidPreset(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestGetPreset(t *testing.T) {
	tests := []struct {
		name      PresetName
		wantSteps int
		wantErr   bool
	}{
		{PresetFast, 2, false},
		{PresetSpecDriven, 1, false},
		{PresetReviewed, 2, false},
		{PresetTDD, 4, false},
		{PresetAutonomous, 3, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.name), func(t *testing.T) {
			got, err := GetPreset(tt.name)
			if tt.wantErr {
				if err == nil {
					t.Error("GetPreset() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("GetPreset() unexpected error: %v", err)
				return
			}
			if len(got.Steps) != tt.wantSteps {
				t.Errorf("GetPreset() returned %d steps, want %d", len(got.Steps), tt.wantSteps)
			}
			// Validate the preset
			if err := got.Validate(); err != nil {
				t.Errorf("GetPreset() returned invalid workflow: %v", err)
			}
		})
	}
}

func TestFastPreset(t *testing.T) {
	w := fastPreset()

	if len(w.Steps) != 2 {
		t.Errorf("fast should have 2 steps, got %d", len(w.Steps))
	}

	// Check implement step
	impl := w.Steps[0]
	if impl.Name != "implement" {
		t.Errorf("first step name = %q, want \"implement\"", impl.Name)
	}
	if !strings.Contains(impl.Prompt, "{{files}}") {
		t.Error("implement prompt should contain {{files}} placeholder")
	}
	if !strings.Contains(impl.Prompt, "Maximise throughput") {
		t.Error("implement prompt should mention maximising throughput")
	}

	// Check review step
	review := w.Steps[1]
	if review.Name != "review" {
		t.Errorf("second step name = %q, want \"review\"", review.Name)
	}
	if !review.Gate {
		t.Error("review step should be a gate")
	}
	if review.OnFail != "implement" {
		t.Errorf("review on_fail = %q, want \"implement\"", review.OnFail)
	}
}

func TestSpecDrivenPreset(t *testing.T) {
	w := specDrivenPreset()

	if len(w.Steps) != 1 {
		t.Errorf("spec-driven should have 1 step, got %d", len(w.Steps))
	}

	step := w.Steps[0]
	if step.Name != "implement" {
		t.Errorf("step name = %q, want \"implement\"", step.Name)
	}
	if !strings.Contains(step.Prompt, "{{files}}") {
		t.Error("step prompt should contain {{files}} placeholder")
	}
	if !strings.Contains(step.Prompt, "<promise>COMPLETE</promise>") {
		t.Error("step prompt should contain completion promise")
	}
}

func TestReviewedPreset(t *testing.T) {
	w := reviewedPreset()

	if len(w.Steps) != 2 {
		t.Errorf("reviewed should have 2 steps, got %d", len(w.Steps))
	}

	// Check implement step
	impl := w.Steps[0]
	if impl.Name != "implement" {
		t.Errorf("first step name = %q, want \"implement\"", impl.Name)
	}
	if impl.Gate {
		t.Error("implement step should not be a gate")
	}

	// Check review step
	review := w.Steps[1]
	if review.Name != "review" {
		t.Errorf("second step name = %q, want \"review\"", review.Name)
	}
	if !review.Gate {
		t.Error("review step should be a gate")
	}
	if review.OnFail != "implement" {
		t.Errorf("review on_fail = %q, want \"implement\"", review.OnFail)
	}
	if !strings.Contains(review.Prompt, "<gate>PASS</gate>") {
		t.Error("review prompt should contain gate pass tag")
	}
	if !strings.Contains(review.Prompt, "<gate>FAIL</gate>") {
		t.Error("review prompt should contain gate fail tag")
	}
}

func TestTDDPreset(t *testing.T) {
	w := tddPreset()

	if len(w.Steps) != 4 {
		t.Errorf("tdd should have 4 steps, got %d", len(w.Steps))
	}

	expectedNames := []string{"red", "green", "refactor", "review"}
	for i, name := range expectedNames {
		if w.Steps[i].Name != name {
			t.Errorf("step %d name = %q, want %q", i, w.Steps[i].Name, name)
		}
	}

	// Check gate configuration
	review := w.Steps[3]
	if !review.Gate {
		t.Error("review step should be a gate")
	}
	if review.OnFail != "refactor" {
		t.Errorf("review on_fail = %q, want \"refactor\"", review.OnFail)
	}
}

func TestValidPresets(t *testing.T) {
	presets := ValidPresets()

	if len(presets) != 5 {
		t.Errorf("ValidPresets() returned %d presets, want 5", len(presets))
	}

	expected := map[PresetName]bool{
		PresetFast:       true,
		PresetSpecDriven: true,
		PresetReviewed:   true,
		PresetTDD:        true,
		PresetAutonomous: true,
	}

	for _, p := range presets {
		if !expected[p] {
			t.Errorf("unexpected preset: %s", p)
		}
	}
}

func TestPresetDescriptions(t *testing.T) {
	descs := PresetDescriptions()

	for _, p := range ValidPresets() {
		if _, ok := descs[p]; !ok {
			t.Errorf("missing description for preset: %s", p)
		}
	}
}

func TestAutonomousPreset(t *testing.T) {
	w := autonomousPreset()

	if len(w.Steps) != 3 {
		t.Errorf("autonomous should have 3 steps, got %d", len(w.Steps))
	}

	// Check implement step
	impl := w.Steps[0]
	if impl.Name != "implement" {
		t.Errorf("first step name = %q, want \"implement\"", impl.Name)
	}
	if !strings.Contains(impl.Prompt, "{{spec_file}}") {
		t.Error("implement prompt should contain {{spec_file}} placeholder")
	}
	if !strings.Contains(impl.Prompt, "{{context_files}}") {
		t.Error("implement prompt should contain {{context_files}} placeholder")
	}
	if !strings.Contains(impl.Prompt, "{{notes_file}}") {
		t.Error("implement prompt should contain {{notes_file}} placeholder")
	}
	if !strings.Contains(impl.Prompt, "highest-leverage") {
		t.Error("implement prompt should mention highest-leverage task selection")
	}
	if !strings.Contains(impl.Prompt, "CONSTRAINTS:") {
		t.Error("implement prompt should contain CONSTRAINTS section")
	}
	if !strings.Contains(impl.Prompt, "ONE task only") {
		t.Error("implement prompt should enforce single-task discipline")
	}
	if impl.Deferred {
		t.Error("implement step should not be deferred")
	}

	// Check fix step
	fix := w.Steps[1]
	if fix.Name != "fix" {
		t.Errorf("second step name = %q, want \"fix\"", fix.Name)
	}
	if !fix.Deferred {
		t.Error("fix step should be deferred")
	}
	if fix.Gate {
		t.Error("fix step should not be a gate")
	}
	if !strings.Contains(fix.Prompt, "review feedback") {
		t.Error("fix prompt should mention review feedback")
	}
	if !strings.Contains(fix.Prompt, "{{notes_file}}") {
		t.Error("fix prompt should contain {{notes_file}} placeholder")
	}
	if !strings.Contains(fix.Prompt, "CONSTRAINTS:") {
		t.Error("fix prompt should contain CONSTRAINTS section")
	}
	if !strings.Contains(fix.Prompt, "Do NOT read the spec file") {
		t.Error("fix prompt should forbid reading spec file for new tasks")
	}
	if !strings.Contains(fix.Prompt, "ONLY address") {
		t.Error("fix prompt should enforce addressing only review issues")
	}

	// Check review step
	review := w.Steps[2]
	if review.Name != "review" {
		t.Errorf("third step name = %q, want \"review\"", review.Name)
	}
	if !review.Gate {
		t.Error("review step should be a gate")
	}
	if review.OnFail != "fix" {
		t.Errorf("review on_fail = %q, want \"fix\"", review.OnFail)
	}
	if !strings.Contains(review.Prompt, "<gate>PASS</gate>") {
		t.Error("review prompt should contain gate pass tag")
	}
	if !strings.Contains(review.Prompt, "<gate>FAIL</gate>") {
		t.Error("review prompt should contain gate fail tag")
	}

	// Validate the preset
	if err := w.Validate(); err != nil {
		t.Errorf("autonomous preset validation failed: %v", err)
	}
}
