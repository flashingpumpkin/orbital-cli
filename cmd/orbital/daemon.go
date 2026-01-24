package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/flashingpumpkin/orbital/internal/daemon"
)

var daemonCmd = &cobra.Command{
	Use:    "daemon",
	Short:  "Daemon management commands",
	Hidden: true, // Internal use - users should use auto-start
}

var daemonStartCmd = &cobra.Command{
	Use:    "start",
	Short:  "Start the orbital daemon",
	Hidden: true,
	RunE:   runDaemonStart,
}

var daemonForeground bool

func init() {
	daemonCmd.AddCommand(daemonStartCmd)
	daemonStartCmd.Flags().BoolVar(&daemonForeground, "foreground", false, "Run in foreground (don't daemonize)")
}

func runDaemonStart(cmd *cobra.Command, args []string) error {
	// Get project directory
	projectDir, err := findProjectDir(workingDir)
	if err != nil {
		return err
	}

	// Check if already running
	if daemon.IsDaemonRunning(projectDir) {
		return fmt.Errorf("daemon already running for project %s", projectDir)
	}

	if daemonForeground {
		// Run in foreground
		return runDaemonForeground(projectDir)
	}

	// Daemonize - start a new process in background
	return startDaemonBackground(projectDir)
}

// runDaemonForeground runs the daemon in the foreground.
func runDaemonForeground(projectDir string) error {
	fmt.Printf("Starting orbital daemon for %s\n", projectDir)

	server := daemon.NewServer(projectDir, nil)
	ctx := context.Background()
	return server.Start(ctx)
}

// startDaemonBackground starts the daemon as a background process.
func startDaemonBackground(projectDir string) error {
	// Get the path to the current executable
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Start daemon process
	cmd := exec.Command(execPath, "daemon", "start", "--foreground", "-d", projectDir)
	cmd.Dir = projectDir

	// Detach from terminal
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Create a new process group
	setSysProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait for daemon to be ready
	client := daemon.NewClient(projectDir)
	for i := 0; i < 50; i++ { // 5 seconds max
		time.Sleep(100 * time.Millisecond)
		if client.IsRunning() {
			return nil
		}
	}

	return fmt.Errorf("daemon failed to start within timeout")
}

// findProjectDir finds the project root by looking for .orbital directory.
func findProjectDir(startDir string) (string, error) {
	if startDir == "" || startDir == "." {
		var err error
		startDir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	// Make absolute
	absDir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	// Walk up looking for .orbital
	dir := absDir
	for {
		orbitalPath := filepath.Join(dir, ".orbital")
		if info, err := os.Stat(orbitalPath); err == nil && info.IsDir() {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding .orbital
			// Use the starting directory
			return absDir, nil
		}
		dir = parent
	}
}

// ensureDaemonRunning ensures a daemon is running for the project.
// If not running, it starts one. Returns the project directory.
func ensureDaemonRunning(startDir string) (string, error) {
	projectDir, err := findProjectDir(startDir)
	if err != nil {
		return "", err
	}

	// Create .orbital directory if it doesn't exist
	orbitalDir := filepath.Join(projectDir, ".orbital")
	if err := os.MkdirAll(orbitalDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .orbital directory: %w", err)
	}

	// Check if daemon is already running
	if daemon.IsDaemonRunning(projectDir) {
		return projectDir, nil
	}

	// Start daemon
	if err := startDaemonBackground(projectDir); err != nil {
		return "", fmt.Errorf("failed to start daemon: %w", err)
	}

	return projectDir, nil
}
