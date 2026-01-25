// Package config provides configuration management for orbit.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/flashingpumpkin/orbital/internal/workflow"
)

// FileConfig represents the configuration loaded from .orbital/config.toml.
type FileConfig struct {
	// Prompt is the custom prompt template. Use {{files}} as placeholder for spec file paths.
	Prompt string `toml:"prompt"`

	// Agents defines custom sub-agents that Claude can delegate to via the Task tool.
	Agents map[string]Agent `toml:"agents"`

	// Workflow defines the multi-step workflow configuration.
	Workflow *WorkflowConfig `toml:"workflow"`

	// Dangerous enables --dangerously-skip-permissions for Claude CLI.
	// When true, Claude can execute commands without prompting for permission.
	// Default is false for safety.
	Dangerous bool `toml:"dangerous"`
}

// WorkflowConfig represents the workflow section in config.toml.
type WorkflowConfig struct {
	// Name is an optional identifier for custom workflows.
	Name string `toml:"name"`

	// Preset is the name of a preset workflow to use.
	Preset string `toml:"preset"`

	// Steps defines the ordered list of workflow steps.
	Steps []workflow.Step `toml:"steps"`

	// MaxGateRetries is the maximum number of times a gate can fail before aborting.
	MaxGateRetries int `toml:"max_gate_retries"`
}

// DefaultPromptTemplate is the default prompt when no config file exists.
const DefaultPromptTemplate = `Implement the user stories in the following spec file{{plural}}:

{{files}}`

// LoadFileConfig reads configuration from .orbital/config.toml in the working directory.
// Returns nil if the file doesn't exist (not an error).
func LoadFileConfig(workingDir string) (*FileConfig, error) {
	configPath := filepath.Join(workingDir, ".orbital", "config.toml")
	return LoadFileConfigFrom(configPath)
}

// LoadFileConfigFrom reads configuration from a specific file path.
// Returns nil if the file doesn't exist (not an error).
func LoadFileConfigFrom(configPath string) (*FileConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var cfg FileConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// BuildPromptFromTemplate builds a prompt using the template and file paths.
func BuildPromptFromTemplate(template string, filePaths []string) string {
	// Handle {{plural}} placeholder
	plural := ""
	if len(filePaths) > 1 {
		plural = "s"
	}
	result := strings.ReplaceAll(template, "{{plural}}", plural)

	// Build file list
	var fileList strings.Builder
	for _, path := range filePaths {
		fileList.WriteString("- ")
		fileList.WriteString(path)
		fileList.WriteString("\n")
	}

	// Replace {{files}} placeholder
	result = strings.ReplaceAll(result, "{{files}}", strings.TrimSuffix(fileList.String(), "\n"))

	return result
}

// ToWorkflow converts WorkflowConfig to a Workflow.
// If a preset is specified and no steps are defined, the preset's steps are used.
// Returns an error if the configuration is invalid.
func (wc *WorkflowConfig) ToWorkflow() (*workflow.Workflow, error) {
	w := &workflow.Workflow{
		Name:           wc.Name,
		Preset:         wc.Preset,
		Steps:          wc.Steps,
		MaxGateRetries: wc.MaxGateRetries,
	}

	// If preset is specified and no custom steps, load preset
	if wc.Preset != "" && len(wc.Steps) == 0 {
		preset, err := workflow.GetPreset(workflow.PresetName(wc.Preset))
		if err != nil {
			return nil, err
		}
		w.Steps = preset.Steps
		if w.Name == "" {
			w.Name = preset.Name
		}
	}

	// Validate the resulting workflow
	if err := w.Validate(); err != nil {
		return nil, err
	}

	return w, nil
}
