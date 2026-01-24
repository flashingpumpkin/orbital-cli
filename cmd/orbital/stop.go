package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/flashingpumpkin/orbital/internal/daemon"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the orbital daemon",
	Long: `Stop the orbital daemon and all running sessions.

Running sessions will be gracefully stopped and their state preserved.
You can resume them later with 'orbital continue' or via the manager TUI.`,
	RunE: runStop,
}

var forceStop bool

func init() {
	stopCmd.Flags().BoolVarP(&forceStop, "force", "f", false, "Force stop even if sessions are running")
}

func runStop(cmd *cobra.Command, args []string) error {
	// Find project directory
	projectDir, err := findProjectDir(workingDir)
	if err != nil {
		return fmt.Errorf("failed to find project directory: %w", err)
	}

	// Check if daemon is running
	if !daemon.IsDaemonRunning(projectDir) {
		fmt.Println("No daemon running for this project.")
		return nil
	}

	// Create client
	client := daemon.NewClient(projectDir)

	// Get status to show what will be stopped
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := client.Status(ctx)
	if err != nil {
		return fmt.Errorf("failed to get daemon status: %w", err)
	}

	if status.Sessions.Running > 0 && !forceStop {
		fmt.Printf("Warning: %d session(s) are still running.\n", status.Sessions.Running)
		fmt.Println("Use --force to stop anyway, or stop sessions individually first.")
		return fmt.Errorf("%d running sessions", status.Sessions.Running)
	}

	// Shutdown daemon
	fmt.Println("Stopping orbital daemon...")
	if err := client.Shutdown(ctx, forceStop); err != nil {
		return fmt.Errorf("failed to stop daemon: %w", err)
	}

	// Wait for daemon to stop
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		if !daemon.IsDaemonRunning(projectDir) {
			fmt.Println("Daemon stopped.")
			return nil
		}
	}

	fmt.Println("Daemon stop requested. It may take a moment to fully shutdown.")
	return nil
}
