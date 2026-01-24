package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/flashingpumpkin/orbital/internal/workflow"
	"github.com/spf13/cobra"
)

// DefaultConfigTemplate is the commented template written by orbit init.
const DefaultConfigTemplate = `# Orbital CLI Configuration
# See: https://github.com/flashingpumpkin/orbital

# Workflow configuration
# Use a preset: spec-driven (default), reviewed, or tdd
# [workflow]
# preset = "spec-driven"

# Or define custom workflow steps:
# [[workflow.steps]]
# name = "implement"
# prompt = "Implement the requirements"
#
# [[workflow.steps]]
# name = "review"
# prompt = "Review the changes"
# gate = true
# on_fail = "implement"

# Custom prompt template for Claude. Uncomment and modify to customise.
# Available placeholders:
#   {{files}}   - List of spec file paths (formatted as "- /path/to/file")
#   {{plural}}  - "s" if multiple files, empty string otherwise
#   {{promise}} - The completion promise string (from --promise flag)
#
# prompt = """
# Implement the user stories in the following spec file{{plural}}:
#
# {{files}}
# """

# Custom agents that Claude can delegate to via the Task tool.
# Each agent needs a description and prompt; tools and model are optional.
#
# [agents.my-agent]
# description = "Brief description shown in agent list"
# prompt = "Detailed instructions for the agent"
# tools = ["Read", "Write", "Bash"]  # optional: restrict available tools
# model = "sonnet"                    # optional: override model for this agent
`

var (
	forceInit  bool
	presetFlag string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a default configuration file",
	Long: `Create a default .orbital/config.toml configuration file.

The configuration file contains commented examples for:
- Workflow configuration (presets or custom steps)
- Custom prompt templates with placeholder documentation
- Custom agent definitions

Available workflow presets:
  fast         Maximise work per iteration with review gate
  spec-driven  Single implement step with completion check (default)
  reviewed     Implement with review gate before completion
  tdd          Red-green-refactor cycle with review gate

If the configuration file already exists, the command will fail unless --force is used.`,
	Args: cobra.NoArgs,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVarP(&forceInit, "force", "f", false, "Overwrite existing configuration file")
	initCmd.Flags().StringVar(&presetFlag, "preset", "", "Workflow preset to use: spec-driven, reviewed, tdd")
}

// newInitCmd creates a new init command for testing.
func newInitCmd() *cobra.Command {
	var force bool
	var preset string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a default configuration file",
		Long: `Create a default .orbital/config.toml configuration file.

The configuration file contains commented examples for:
- Workflow configuration (presets or custom steps)
- Custom prompt templates with placeholder documentation
- Custom agent definitions

If the configuration file already exists, the command will fail unless --force is used.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInitWithOptions(cmd, force, preset)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing configuration file")
	cmd.Flags().StringVar(&preset, "preset", "", "Workflow preset to use: spec-driven, reviewed, tdd")
	return cmd
}

func runInitWithOptions(cmd *cobra.Command, force bool, preset string) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	orbitDir := filepath.Join(workingDir, ".orbital")
	configPath := filepath.Join(orbitDir, "config.toml")

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil && !force {
		return fmt.Errorf("configuration file already exists: %s (use --force to overwrite)", configPath)
	}

	// Validate preset if specified
	if preset != "" && !workflow.IsValidPreset(preset) {
		validPresets := workflow.ValidPresets()
		names := make([]string, len(validPresets))
		for i, p := range validPresets {
			names[i] = string(p)
		}
		return fmt.Errorf("invalid preset %q, valid options: %s", preset, strings.Join(names, ", "))
	}

	// Create .orbital directory if it doesn't exist
	if err := os.MkdirAll(orbitDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", orbitDir, err)
	}

	// Generate config content
	configContent := generateConfigContent(preset)

	// Write the config file
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Created %s\n", configPath)
	if preset != "" {
		_, _ = fmt.Fprintf(out, "Using workflow preset: %s\n", preset)
	}

	return nil
}

func runInit(cmd *cobra.Command, args []string) error {
	return runInitWithOptions(cmd, forceInit, presetFlag)
}

// generateConfigContent generates the config file content with optional preset.
func generateConfigContent(preset string) string {
	if preset == "" {
		return DefaultConfigTemplate
	}

	// Get the preset workflow
	w, err := workflow.GetPreset(workflow.PresetName(preset))
	if err != nil {
		// Fallback to just the preset name if something goes wrong
		return generateConfigWithPresetName(preset)
	}

	// Generate config with full workflow steps
	var sb strings.Builder
	sb.WriteString(`# Orbital CLI Configuration
# See: https://github.com/flashingpumpkin/orbital

# Workflow configuration (`)
	sb.WriteString(preset)
	sb.WriteString(` preset)
# Modify these steps to customise the workflow.

[workflow]
name = "`)
	sb.WriteString(preset)
	sb.WriteString(`"

`)

	// Write each step
	for _, step := range w.Steps {
		sb.WriteString("[[workflow.steps]]\n")
		sb.WriteString(`name = "`)
		sb.WriteString(step.Name)
		sb.WriteString("\"\n")
		sb.WriteString(`prompt = """
`)
		sb.WriteString(step.Prompt)
		sb.WriteString("\n\"\"\"\n")
		if step.Gate {
			sb.WriteString("gate = true\n")
		}
		if step.OnFail != "" {
			sb.WriteString(`on_fail = "`)
			sb.WriteString(step.OnFail)
			sb.WriteString("\"\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`# Custom prompt template for Claude. Uncomment and modify to customise.
# Available placeholders:
#   {{files}}   - List of spec file paths (formatted as "- /path/to/file")
#   {{plural}}  - "s" if multiple files, empty string otherwise
#   {{promise}} - The completion promise string (from --promise flag)
#
# prompt = """
# Implement the user stories in the following spec file{{plural}}:
#
# {{files}}
# """

# Custom agents that Claude can delegate to via the Task tool.
# Each agent needs a description and prompt; tools and model are optional.
#
# [agents.my-agent]
# description = "Brief description shown in agent list"
# prompt = "Detailed instructions for the agent"
# tools = ["Read", "Write", "Bash"]  # optional: restrict available tools
# model = "sonnet"                    # optional: override model for this agent
`)
	return sb.String()
}

// generateConfigWithPresetName generates config with just the preset name (fallback).
func generateConfigWithPresetName(preset string) string {
	var sb strings.Builder
	sb.WriteString(`# Orbital CLI Configuration
# See: https://github.com/flashingpumpkin/orbital

# Workflow configuration
[workflow]
preset = "`)
	sb.WriteString(preset)
	sb.WriteString(`"

# Custom prompt template for Claude. Uncomment and modify to customise.
# Available placeholders:
#   {{files}}   - List of spec file paths (formatted as "- /path/to/file")
#   {{plural}}  - "s" if multiple files, empty string otherwise
#   {{promise}} - The completion promise string (from --promise flag)
#
# prompt = """
# Implement the user stories in the following spec file{{plural}}:
#
# {{files}}
# """

# Custom agents that Claude can delegate to via the Task tool.
# Each agent needs a description and prompt; tools and model are optional.
#
# [agents.my-agent]
# description = "Brief description shown in agent list"
# prompt = "Detailed instructions for the agent"
# tools = ["Read", "Write", "Bash"]  # optional: restrict available tools
# model = "sonnet"                    # optional: override model for this agent
`)
	return sb.String()
}
