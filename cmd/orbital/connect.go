package main

import (
	"fmt"

	"github.com/flashingpumpkin/orbital/internal/daemon"
	"github.com/flashingpumpkin/orbital/internal/dtui"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to the orbital daemon manager TUI",
	Long: `Connect to the orbital daemon to view and manage all running sessions.

The manager TUI provides:
- Tree view of all sessions grouped by status
- Real-time progress updates
- Session output viewing
- Interactive chat
- Worktree merge control

If no daemon is running, this command will fail with an error.
Start a session with 'orbital <spec>' to auto-start the daemon.`,
	RunE: runConnect,
}

func init() {
	// No additional flags needed
}

func runConnect(cmd *cobra.Command, args []string) error {
	// Find project directory
	projectDir, err := findProjectDir(workingDir)
	if err != nil {
		return fmt.Errorf("failed to find project directory: %w", err)
	}

	// Check if daemon is running
	if !daemon.IsDaemonRunning(projectDir) {
		return fmt.Errorf("no daemon running for project %s\nStart a session with 'orbital <spec>' to auto-start the daemon", projectDir)
	}

	// Create client
	client := daemon.NewClient(projectDir)

	// Launch the manager TUI
	return dtui.Run(client, projectDir)
}
