package config

import (
	"testing"
	"time"
)

func TestNewConfig_ReturnsConfigWithDefaults(t *testing.T) {
	cfg := NewConfig()

	if cfg == nil {
		t.Fatal("NewConfig() returned nil")
	}

	// Check default values
	if cfg.MaxIterations != 50 {
		t.Errorf("MaxIterations = %d; want 50", cfg.MaxIterations)
	}

	if cfg.CompletionPromise != "<promise>COMPLETE</promise>" {
		t.Errorf("CompletionPromise = %q; want %q", cfg.CompletionPromise, "<promise>COMPLETE</promise>")
	}

	if cfg.Model != "opus" {
		t.Errorf("Model = %q; want %q", cfg.Model, "opus")
	}

	if cfg.MaxBudget != 100.00 {
		t.Errorf("MaxBudget = %f; want 100.00", cfg.MaxBudget)
	}

	if cfg.WorkingDir != "." {
		t.Errorf("WorkingDir = %q; want %q", cfg.WorkingDir, ".")
	}

	if cfg.IterationTimeout != 30*time.Minute {
		t.Errorf("IterationTimeout = %v; want %v", cfg.IterationTimeout, 30*time.Minute)
	}

	// Check zero values for non-defaulted fields
	if cfg.SpecPath != "" {
		t.Errorf("SpecPath = %q; want empty string", cfg.SpecPath)
	}

	if cfg.Verbose != false {
		t.Errorf("Verbose = %t; want false", cfg.Verbose)
	}

	if cfg.DryRun != false {
		t.Errorf("DryRun = %t; want false", cfg.DryRun)
	}

	if cfg.SessionID != "" {
		t.Errorf("SessionID = %q; want empty string", cfg.SessionID)
	}
}

func TestConfig_Validate_ReturnsErrorWhenSpecPathEmpty(t *testing.T) {
	cfg := NewConfig()
	cfg.SpecPath = ""

	err := cfg.Validate()

	if err == nil {
		t.Error("Validate() returned nil; want error for empty SpecPath")
	}

	expectedMsg := "spec path is required"
	if err.Error() != expectedMsg {
		t.Errorf("error message = %q; want %q", err.Error(), expectedMsg)
	}
}

func TestConfig_Validate_ReturnsNilWhenSpecPathSet(t *testing.T) {
	cfg := NewConfig()
	cfg.SpecPath = "/path/to/spec.md"

	err := cfg.Validate()

	if err != nil {
		t.Errorf("Validate() returned error %v; want nil", err)
	}
}

func TestConfig_Validate_AcceptsRelativeSpecPath(t *testing.T) {
	cfg := NewConfig()
	cfg.SpecPath = "specs/my-spec.md"

	err := cfg.Validate()

	if err != nil {
		t.Errorf("Validate() returned error %v; want nil for relative path", err)
	}
}

func TestConfig_FieldsAreSettable(t *testing.T) {
	cfg := NewConfig()

	// Set all fields
	cfg.SpecPath = "/custom/spec.md"
	cfg.MaxIterations = 100
	cfg.CompletionPromise = "DONE"
	cfg.Model = "opus"
	cfg.MaxBudget = 25.50
	cfg.WorkingDir = "/custom/dir"
	cfg.Verbose = true
	cfg.DryRun = true
	cfg.SessionID = "test-session-123"
	cfg.IterationTimeout = 45 * time.Minute

	// Verify all fields
	if cfg.SpecPath != "/custom/spec.md" {
		t.Errorf("SpecPath = %q; want %q", cfg.SpecPath, "/custom/spec.md")
	}
	if cfg.MaxIterations != 100 {
		t.Errorf("MaxIterations = %d; want 100", cfg.MaxIterations)
	}
	if cfg.CompletionPromise != "DONE" {
		t.Errorf("CompletionPromise = %q; want %q", cfg.CompletionPromise, "DONE")
	}
	if cfg.Model != "opus" {
		t.Errorf("Model = %q; want %q", cfg.Model, "opus")
	}
	if cfg.MaxBudget != 25.50 {
		t.Errorf("MaxBudget = %f; want 25.50", cfg.MaxBudget)
	}
	if cfg.WorkingDir != "/custom/dir" {
		t.Errorf("WorkingDir = %q; want %q", cfg.WorkingDir, "/custom/dir")
	}
	if cfg.Verbose != true {
		t.Errorf("Verbose = %t; want true", cfg.Verbose)
	}
	if cfg.DryRun != true {
		t.Errorf("DryRun = %t; want true", cfg.DryRun)
	}
	if cfg.SessionID != "test-session-123" {
		t.Errorf("SessionID = %q; want %q", cfg.SessionID, "test-session-123")
	}
	if cfg.IterationTimeout != 45*time.Minute {
		t.Errorf("IterationTimeout = %v; want %v", cfg.IterationTimeout, 45*time.Minute)
	}
}
