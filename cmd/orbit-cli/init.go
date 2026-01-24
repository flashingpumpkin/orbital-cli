package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// DefaultConfigTemplate is the commented template written by orbit init.
const DefaultConfigTemplate = `# Orbit CLI Configuration
# See: https://github.com/flashingpumpkin/orbit-cli

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

var forceInit bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a default configuration file",
	Long: `Create a default .orbit/config.toml configuration file.

The configuration file contains commented examples for:
- Custom prompt templates with placeholder documentation
- Custom agent definitions

If the configuration file already exists, the command will fail unless --force is used.`,
	Args: cobra.NoArgs,
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVarP(&forceInit, "force", "f", false, "Overwrite existing configuration file")
}

// newInitCmd creates a new init command for testing.
func newInitCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a default configuration file",
		Long: `Create a default .orbit/config.toml configuration file.

The configuration file contains commented examples for:
- Custom prompt templates with placeholder documentation
- Custom agent definitions

If the configuration file already exists, the command will fail unless --force is used.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInitWithForce(cmd, force)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing configuration file")
	return cmd
}

func runInitWithForce(cmd *cobra.Command, force bool) error {
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	orbitDir := filepath.Join(workingDir, ".orbit")
	configPath := filepath.Join(orbitDir, "config.toml")

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil && !force {
		return fmt.Errorf("configuration file already exists: %s (use --force to overwrite)", configPath)
	}

	// Create .orbit directory if it doesn't exist
	if err := os.MkdirAll(orbitDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", orbitDir, err)
	}

	// Write the config file
	if err := os.WriteFile(configPath, []byte(DefaultConfigTemplate), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Created %s\n", configPath)

	return nil
}

func runInit(cmd *cobra.Command, args []string) error {
	return runInitWithForce(cmd, forceInit)
}
