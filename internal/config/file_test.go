package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flashingpumpkin/orbit-cli/internal/workflow"
)

func TestLoadFileConfig_NotExists(t *testing.T) {
	cfg, err := LoadFileConfig("/nonexistent/path")
	if err != nil {
		t.Fatalf("LoadFileConfig() error = %v, want nil", err)
	}
	if cfg != nil {
		t.Errorf("LoadFileConfig() = %v, want nil", cfg)
	}
}

func TestLoadFileConfig_ValidConfig(t *testing.T) {
	// Create temp directory with config
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".orbit")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	configContent := `prompt = "Custom prompt with {{files}}"`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFileConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadFileConfig() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadFileConfig() = nil, want config")
	}
	if cfg.Prompt != "Custom prompt with {{files}}" {
		t.Errorf("Prompt = %q, want %q", cfg.Prompt, "Custom prompt with {{files}}")
	}
}

func TestLoadFileConfig_InvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".orbit")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("invalid toml {{{"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFileConfig(tmpDir)
	if err == nil {
		t.Error("LoadFileConfig() error = nil, want error for invalid TOML")
	}
}

func TestLoadFileConfig_WithAgents(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".orbit")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	configContent := `prompt = "Custom prompt"

[agents.reviewer]
description = "Code review specialist"
prompt = "You are a code reviewer"

[agents.implementor]
description = "Implementation specialist"
prompt = "You are a senior developer"
tools = ["Read", "Write", "Bash"]
model = "opus"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFileConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadFileConfig() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadFileConfig() = nil, want config")
	}
	if cfg.Prompt != "Custom prompt" {
		t.Errorf("Prompt = %q, want %q", cfg.Prompt, "Custom prompt")
	}
	if len(cfg.Agents) != 2 {
		t.Fatalf("len(Agents) = %d, want 2", len(cfg.Agents))
	}

	// Check reviewer agent
	reviewer, ok := cfg.Agents["reviewer"]
	if !ok {
		t.Fatal("Agents['reviewer'] not found")
	}
	if reviewer.Description != "Code review specialist" {
		t.Errorf("reviewer.Description = %q, want %q", reviewer.Description, "Code review specialist")
	}
	if reviewer.Prompt != "You are a code reviewer" {
		t.Errorf("reviewer.Prompt = %q, want %q", reviewer.Prompt, "You are a code reviewer")
	}

	// Check implementor agent with optional fields
	impl, ok := cfg.Agents["implementor"]
	if !ok {
		t.Fatal("Agents['implementor'] not found")
	}
	if impl.Model != "opus" {
		t.Errorf("implementor.Model = %q, want %q", impl.Model, "opus")
	}
	if len(impl.Tools) != 3 {
		t.Fatalf("len(implementor.Tools) = %d, want 3", len(impl.Tools))
	}
}

func TestLoadFileConfig_AgentsOnly(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".orbit")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Config with agents but no custom prompt
	configContent := `[agents.tester]
description = "Test specialist"
prompt = "You run tests"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFileConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadFileConfig() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadFileConfig() = nil, want config")
	}
	if cfg.Prompt != "" {
		t.Errorf("Prompt = %q, want empty", cfg.Prompt)
	}
	if len(cfg.Agents) != 1 {
		t.Fatalf("len(Agents) = %d, want 1", len(cfg.Agents))
	}
}

func TestBuildPromptFromTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		files    []string
		want     string
	}{
		{
			name:     "single file",
			template: "Process {{files}}",
			files:    []string{"/path/to/file.md"},
			want:     "Process - /path/to/file.md",
		},
		{
			name:     "multiple files",
			template: "Process file{{plural}}:\n{{files}}",
			files:    []string{"/path/a.md", "/path/b.md"},
			want:     "Process files:\n- /path/a.md\n- /path/b.md",
		},
		{
			name:     "plural with single file",
			template: "File{{plural}}: {{files}}",
			files:    []string{"/only.md"},
			want:     "File: - /only.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPromptFromTemplate(tt.template, tt.files)
			if got != tt.want {
				t.Errorf("BuildPromptFromTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadFileConfig_WithWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".orbit")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	configContent := `[workflow]
name = "my-workflow"

[[workflow.steps]]
name = "implement"
prompt = "Implement the feature"

[[workflow.steps]]
name = "review"
prompt = "Review the changes"
gate = true
on_fail = "implement"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFileConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadFileConfig() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadFileConfig() = nil, want config")
	}
	if cfg.Workflow == nil {
		t.Fatal("Workflow = nil, want workflow config")
	}
	if cfg.Workflow.Name != "my-workflow" {
		t.Errorf("Workflow.Name = %q, want %q", cfg.Workflow.Name, "my-workflow")
	}
	if len(cfg.Workflow.Steps) != 2 {
		t.Fatalf("len(Workflow.Steps) = %d, want 2", len(cfg.Workflow.Steps))
	}

	// Check first step
	step1 := cfg.Workflow.Steps[0]
	if step1.Name != "implement" {
		t.Errorf("Steps[0].Name = %q, want \"implement\"", step1.Name)
	}

	// Check second step (gate)
	step2 := cfg.Workflow.Steps[1]
	if step2.Name != "review" {
		t.Errorf("Steps[1].Name = %q, want \"review\"", step2.Name)
	}
	if !step2.Gate {
		t.Error("Steps[1].Gate = false, want true")
	}
	if step2.OnFail != "implement" {
		t.Errorf("Steps[1].OnFail = %q, want \"implement\"", step2.OnFail)
	}
}

func TestLoadFileConfig_WithPreset(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".orbit")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	configContent := `[workflow]
preset = "tdd"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFileConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadFileConfig() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadFileConfig() = nil, want config")
	}
	if cfg.Workflow == nil {
		t.Fatal("Workflow = nil, want workflow config")
	}
	if cfg.Workflow.Preset != "tdd" {
		t.Errorf("Workflow.Preset = %q, want \"tdd\"", cfg.Workflow.Preset)
	}
}

func TestWorkflowConfig_ToWorkflow(t *testing.T) {
	tests := []struct {
		name      string
		config    WorkflowConfig
		wantSteps int
		wantErr   bool
	}{
		{
			name: "custom steps",
			config: WorkflowConfig{
				Name: "custom",
				Steps: []workflow.Step{
					{Name: "implement", Prompt: "Do it"},
				},
			},
			wantSteps: 1,
			wantErr:   false,
		},
		{
			name: "preset expands",
			config: WorkflowConfig{
				Preset: "tdd",
			},
			wantSteps: 4, // TDD has 4 steps
			wantErr:   false,
		},
		{
			name: "invalid preset",
			config: WorkflowConfig{
				Preset: "nonexistent",
			},
			wantSteps: 0,
			wantErr:   true,
		},
		{
			name: "custom steps with preset - custom takes precedence",
			config: WorkflowConfig{
				Preset: "tdd",
				Steps: []workflow.Step{
					{Name: "only", Prompt: "Only step"},
				},
			},
			wantSteps: 1, // Custom steps, preset ignored
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, err := tt.config.ToWorkflow()
			if tt.wantErr {
				if err == nil {
					t.Error("ToWorkflow() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("ToWorkflow() unexpected error: %v", err)
				return
			}
			if len(w.Steps) != tt.wantSteps {
				t.Errorf("ToWorkflow() returned %d steps, want %d", len(w.Steps), tt.wantSteps)
			}
		})
	}
}
