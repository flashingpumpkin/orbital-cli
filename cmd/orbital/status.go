package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/flashingpumpkin/orbital/internal/daemon"
	"github.com/flashingpumpkin/orbital/internal/state"
)

var statusJSON bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display session status",
	Long: `Display the status of orbital sessions.

When a daemon is running, shows all sessions managed by the daemon.
Otherwise, shows the local session state (if any).

Use --json for machine-readable output.`,
	Args: cobra.NoArgs,
	RunE: runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output in JSON format")
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Find project directory
	projectDir, err := findProjectDir(workingDir)
	if err != nil {
		projectDir, _ = os.Getwd()
	}

	out := cmd.OutOrStdout()

	// Check if daemon is running
	if daemon.IsDaemonRunning(projectDir) {
		return runDaemonStatus(projectDir, out)
	}

	// Fall back to legacy local status
	return runLocalStatus(projectDir, out)
}

// runDaemonStatus shows status from the running daemon.
func runDaemonStatus(projectDir string, out io.Writer) error {
	client := daemon.NewClient(projectDir)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Get daemon status
	status, err := client.Status(ctx)
	if err != nil {
		return fmt.Errorf("failed to get daemon status: %w", err)
	}

	// Get all sessions
	sessions, err := client.ListSessions(ctx)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if statusJSON {
		data := map[string]interface{}{
			"daemon":   status,
			"sessions": sessions,
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}

	// Print formatted output
	fmt.Fprintf(out, "Orbital Daemon (PID %d)\n", status.PID)
	fmt.Fprintf(out, "Project: %s\n", status.ProjectDir)
	fmt.Fprintf(out, "Started: %s\n", status.StartedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintln(out)

	// Group sessions by status
	running := filterSessions(sessions, daemon.StatusRunning, daemon.StatusMerging)
	completed := filterSessions(sessions, daemon.StatusCompleted, daemon.StatusMerged)
	failed := filterSessions(sessions, daemon.StatusFailed, daemon.StatusConflict)
	stopped := filterSessions(sessions, daemon.StatusStopped, daemon.StatusInterrupted)

	if len(running) > 0 {
		fmt.Fprintf(out, "RUNNING (%d)\n", len(running))
		for _, s := range running {
			printSessionLine(out, s)
		}
		fmt.Fprintln(out)
	}

	if len(completed) > 0 {
		fmt.Fprintf(out, "COMPLETED (%d)\n", len(completed))
		for _, s := range completed {
			printSessionLine(out, s)
		}
		fmt.Fprintln(out)
	}

	if len(failed) > 0 {
		fmt.Fprintf(out, "FAILED (%d)\n", len(failed))
		for _, s := range failed {
			printSessionLine(out, s)
		}
		fmt.Fprintln(out)
	}

	if len(stopped) > 0 {
		fmt.Fprintf(out, "STOPPED (%d)\n", len(stopped))
		for _, s := range stopped {
			printSessionLine(out, s)
		}
		fmt.Fprintln(out)
	}

	if len(sessions) == 0 {
		fmt.Fprintln(out, "No sessions. Start one with 'orbital <spec-file>'")
		fmt.Fprintln(out)
	}

	fmt.Fprintf(out, "Total cost: $%.2f\n", status.TotalCost)
	return nil
}

// filterSessions returns sessions matching any of the given statuses.
func filterSessions(sessions []*daemon.Session, statuses ...daemon.SessionStatus) []*daemon.Session {
	var result []*daemon.Session
	for _, s := range sessions {
		for _, status := range statuses {
			if s.Status == status {
				result = append(result, s)
				break
			}
		}
	}
	return result
}

// printSessionLine prints a single session status line.
func printSessionLine(out io.Writer, s *daemon.Session) {
	specName := "unknown"
	if len(s.SpecFiles) > 0 {
		specName = filepath.Base(s.SpecFiles[0])
	}

	timeAgo := formatTimeAgo(s.StartedAt)

	worktree := ""
	if s.Worktree != nil {
		worktree = fmt.Sprintf(" [%s]", s.Worktree.Branch)
	}

	fmt.Fprintf(out, "  %-30s iter %d/%-4d $%-8.2f %s%s\n",
		truncateString(specName, 30),
		s.Iteration,
		s.MaxIterations,
		s.TotalCost,
		timeAgo,
		worktree,
	)
}

// formatTimeAgo formats a time as a human-readable relative string.
func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// truncateString truncates a string to the given length.
func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length-3] + "..."
}

// runLocalStatus shows status from local state files (legacy mode).
func runLocalStatus(projectDir string, out io.Writer) error {
	stateDir := state.StateDir(projectDir)

	// Try to load state
	var st *state.State
	var isRunning bool
	var err error
	if state.Exists(projectDir) {
		st, err = state.Load(projectDir)
		if err != nil {
			return fmt.Errorf("failed to load state: %w", err)
		}
		isRunning = !st.IsStale()
	}

	// Try to load queue
	queue, _ := state.LoadQueue(stateDir)

	// Check if there's anything to show
	hasState := st != nil
	hasQueue := queue != nil && !queue.IsEmpty()

	if !hasState && !hasQueue {
		fmt.Fprintln(out, "No orbital session in this directory")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Start with: orbital <spec-file>")
		fmt.Fprintln(out, "Or use --daemon flag to run via daemon")
		return nil
	}

	if statusJSON {
		data := map[string]interface{}{
			"state":   st,
			"queue":   queue,
			"running": isRunning,
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}

	// Print header
	fmt.Fprintln(out, "Orbital Status (local)")
	fmt.Fprintln(out, "=====================")

	// Print status indicator
	if isRunning {
		fmt.Fprintln(out, "Status:     RUNNING")
	} else if hasState {
		fmt.Fprintln(out, "Status:     STOPPED (run 'orbital continue' to resume)")
	} else {
		fmt.Fprintln(out, "Status:     PENDING (queued files waiting)")
	}

	// Print state info if available
	if hasState {
		fmt.Fprintf(out, "PID:        %d\n", st.PID)
		fmt.Fprintf(out, "Session:    %s\n", st.SessionID)
		fmt.Fprintf(out, "Iteration:  %d\n", st.Iteration)
		fmt.Fprintf(out, "Cost:       $%.2f USD\n", st.TotalCost)
		fmt.Fprintf(out, "Started:    %s\n", st.StartedAt.Format("2006-01-02 15:04:05"))
	}
	fmt.Fprintln(out)

	// Print active files
	if hasState && len(st.ActiveFiles) > 0 {
		fmt.Fprintln(out, "Active Files:")
		for _, f := range st.ActiveFiles {
			fmt.Fprintf(out, "  - %s\n", f)
		}
		fmt.Fprintln(out)
	}

	// Print queued files
	if hasQueue {
		fmt.Fprintln(out, "Queued Files:")
		for _, f := range queue.QueuedFiles {
			addedAt, ok := queue.AddedAt[f]
			if ok {
				ago := formatDuration(time.Since(addedAt))
				fmt.Fprintf(out, "  - %s (added %s ago)\n", f, ago)
			} else {
				fmt.Fprintf(out, "  - %s\n", f)
			}
		}
	} else if hasState {
		fmt.Fprintln(out, "Queued Files: (none)")
	}

	return nil
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// newStatusCmd is no longer needed - kept for backwards compatibility
func newStatusCmd() *cobra.Command {
	return statusCmd
}
