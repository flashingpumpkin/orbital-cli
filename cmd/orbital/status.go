package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/flashingpumpkin/orbital/internal/state"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Display the current session state",
	Long: `Display the current orbital session state.

Shows information about the running instance including:
- Process ID (PID)
- Session ID
- Current iteration count
- Total cost
- Active files being processed
- Files queued for processing`,
	Args: cobra.NoArgs,
	RunE: runStatus,
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Display the current session state",
		Long: `Display the current orbital session state.

Shows information about the running instance including:
- Process ID (PID)
- Session ID
- Current iteration count
- Total cost
- Active files being processed
- Files queued for processing`,
		Args: cobra.NoArgs,
		RunE: runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Get current working directory
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	out := cmd.OutOrStdout()
	stateDir := state.StateDir(workingDir)

	// Try to load state
	var st *state.State
	var isRunning bool
	if state.Exists(workingDir) {
		st, err = state.Load(workingDir)
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
		_, _ = fmt.Fprintln(out, "No orbital session in this directory")
		_, _ = fmt.Fprintln(out, "")
		_, _ = fmt.Fprintln(out, "Start with: orbital <spec-file>")
		return nil
	}

	// Print header
	_, _ = fmt.Fprintln(out, "Orbital Status")
	_, _ = fmt.Fprintln(out, "=============")

	// Print status indicator
	if isRunning {
		_, _ = fmt.Fprintln(out, "Status:     RUNNING")
	} else if hasState {
		_, _ = fmt.Fprintln(out, "Status:     STOPPED (run 'orbital continue' to resume)")
	} else {
		_, _ = fmt.Fprintln(out, "Status:     PENDING (queued files waiting)")
	}

	// Print state info if available
	if hasState {
		_, _ = fmt.Fprintf(out, "PID:        %d\n", st.PID)
		_, _ = fmt.Fprintf(out, "Session:    %s\n", st.SessionID)
		_, _ = fmt.Fprintf(out, "Iteration:  %d\n", st.Iteration)
		_, _ = fmt.Fprintf(out, "Cost:       $%.2f USD\n", st.TotalCost)
		_, _ = fmt.Fprintf(out, "Started:    %s\n", st.StartedAt.Format("2006-01-02 15:04:05"))
	}
	_, _ = fmt.Fprintln(out)

	// Print active files
	if hasState && len(st.ActiveFiles) > 0 {
		_, _ = fmt.Fprintln(out, "Active Files:")
		for _, f := range st.ActiveFiles {
			_, _ = fmt.Fprintf(out, "  - %s\n", f)
		}
		_, _ = fmt.Fprintln(out)
	}

	// Print queued files
	if hasQueue {
		_, _ = fmt.Fprintln(out, "Queued Files:")
		for _, f := range queue.QueuedFiles {
			addedAt, ok := queue.AddedAt[f]
			if ok {
				ago := formatDuration(time.Since(addedAt))
				_, _ = fmt.Fprintf(out, "  - %s (added %s ago)\n", f, ago)
			} else {
				_, _ = fmt.Fprintf(out, "  - %s\n", f)
			}
		}
	} else if hasState {
		_, _ = fmt.Fprintln(out, "Queued Files: (none)")
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
