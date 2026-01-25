package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/flashingpumpkin/orbital/internal/worktree"
)

var (
	jsonOutput     bool
	forceRemove    bool
	dryRunCleanup  bool
	forceCleanup   bool
)

var worktreeCmd = &cobra.Command{
	Use:   "worktree",
	Short: "Manage orbital worktrees",
	Long: `Manage orbital worktrees.

Orbital can run in isolated git worktrees to prevent changes from affecting
your main branch until work is complete. This command group provides tools
for inspecting, removing, and cleaning up worktrees.`,
}

var worktreeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tracked worktrees",
	Long: `List all tracked orbital worktrees with their status.

Displays worktree name, path, branch, and current status:
- ACTIVE: Worktree exists and is valid
- MISSING: Worktree path does not exist on disk
- ORPHAN: Git worktree not registered with git`,
	RunE: runWorktreeList,
}

var worktreeShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show detailed status of a worktree",
	Long: `Show detailed information about a specific worktree.

Displays:
- Basic information (path, branch, original branch)
- Git status (uncommitted changes)
- Commit divergence from original branch`,
	Args: cobra.ExactArgs(1),
	RunE: runWorktreeShow,
}

var worktreeRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a worktree",
	Long: `Remove a specific worktree and its branch.

This abandons any uncommitted work in the worktree. Use --force to skip
the confirmation prompt.`,
	Args: cobra.ExactArgs(1),
	RunE: runWorktreeRemove,
}

var worktreeCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Find and remove orphan worktrees",
	Long: `Find and remove orphaned worktrees and branches.

Detects:
- State entries for non-existent paths
- Git worktrees not tracked in orbital state
- Orphan orbital/* branches

Use --dry-run to see what would be cleaned without making changes.
Use --force to skip confirmation prompts.`,
	RunE: runWorktreeCleanup,
}

func init() {
	// List command flags
	worktreeListCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	// Remove command flags
	worktreeRemoveCmd.Flags().BoolVarP(&forceRemove, "force", "f", false, "Skip confirmation prompt")

	// Cleanup command flags
	worktreeCleanupCmd.Flags().BoolVar(&dryRunCleanup, "dry-run", false, "Show what would be cleaned without making changes")
	worktreeCleanupCmd.Flags().BoolVarP(&forceCleanup, "force", "f", false, "Skip confirmation prompts")

	// Add subcommands
	worktreeCmd.AddCommand(worktreeListCmd)
	worktreeCmd.AddCommand(worktreeShowCmd)
	worktreeCmd.AddCommand(worktreeRemoveCmd)
	worktreeCmd.AddCommand(worktreeCleanupCmd)
}

// WorktreeListEntry represents a worktree for list output.
type WorktreeListEntry struct {
	Name           string   `json:"name"`
	Path           string   `json:"path"`
	Branch         string   `json:"branch"`
	OriginalBranch string   `json:"originalBranch"`
	SpecFiles      []string `json:"specFiles"`
	Status         string   `json:"status"`
}

func runWorktreeList(cmd *cobra.Command, args []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	out := cmd.OutOrStdout()
	stateManager := worktree.NewStateManager(wd)
	worktrees, err := stateManager.List()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	if len(worktrees) == 0 {
		if jsonOutput {
			fmt.Fprintln(out, "[]")
		} else {
			fmt.Fprintln(out, "No active worktrees")
		}
		return nil
	}

	// Build list entries with status
	entries := make([]WorktreeListEntry, 0, len(worktrees))
	for _, wt := range worktrees {
		status := getWorktreeStatus(wd, &wt)
		entry := WorktreeListEntry{
			Name:           wt.Name,
			Path:           wt.Path,
			Branch:         wt.Branch,
			OriginalBranch: wt.OriginalBranch,
			SpecFiles:      wt.SpecFiles,
			Status:         status,
		}
		entries = append(entries, entry)
	}

	if jsonOutput {
		output, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Fprintln(out, string(output))
		return nil
	}

	// Print table format
	fmt.Fprintln(out, "Orbital Worktrees")
	fmt.Fprintln(out, "=================")
	fmt.Fprintln(out)
	for _, entry := range entries {
		statusIndicator := ""
		switch entry.Status {
		case "ACTIVE":
			statusIndicator = "[ACTIVE]"
		case "MISSING":
			statusIndicator = "[MISSING]"
		case "ORPHAN":
			statusIndicator = "[ORPHAN]"
		}
		fmt.Fprintf(out, "Name:     %s %s\n", entry.Name, statusIndicator)
		fmt.Fprintf(out, "Path:     %s\n", entry.Path)
		fmt.Fprintf(out, "Branch:   %s -> %s\n", entry.Branch, entry.OriginalBranch)
		if len(entry.SpecFiles) > 0 {
			fmt.Fprintf(out, "Specs:    %s\n", strings.Join(entry.SpecFiles, ", "))
		}
		fmt.Fprintln(out)
	}

	return nil
}

func runWorktreeShow(cmd *cobra.Command, args []string) error {
	name := args[0]

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	stateManager := worktree.NewStateManager(wd)
	wt, err := stateManager.FindByName(name)
	if err != nil {
		return fmt.Errorf("failed to find worktree: %w", err)
	}
	if wt == nil {
		return fmt.Errorf("worktree not found: %s", name)
	}

	out := cmd.OutOrStdout()

	// Basic info
	fmt.Fprintln(out, "Worktree Details")
	fmt.Fprintln(out, "================")
	fmt.Fprintf(out, "Name:            %s\n", wt.Name)
	fmt.Fprintf(out, "Path:            %s\n", wt.Path)
	fmt.Fprintf(out, "Branch:          %s\n", wt.Branch)
	fmt.Fprintf(out, "Original Branch: %s\n", wt.OriginalBranch)
	fmt.Fprintf(out, "Created:         %s\n", wt.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Fprintln(out)

	// Check status
	status := getWorktreeStatus(wd, wt)
	fmt.Fprintf(out, "Status:          %s\n", status)
	if status == "MISSING" {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "The worktree directory does not exist.")
		fmt.Fprintln(out, "Run 'orbital worktree cleanup' to remove stale entries.")
		return nil
	}

	// Git status (only if path exists)
	if status == "ACTIVE" {
		fmt.Fprintln(out)

		// Get uncommitted changes
		gitStatus, err := getGitStatus(wt.Path)
		if err != nil {
			fmt.Fprintf(out, "Git Status:      (error: %v)\n", err)
		} else if gitStatus == "" {
			fmt.Fprintln(out, "Git Status:      Clean (no uncommitted changes)")
		} else {
			fmt.Fprintln(out, "Git Status:      Uncommitted changes")
			fmt.Fprintln(out, gitStatus)
		}

		// Get divergence
		ahead, behind, err := getBranchDivergence(wd, wt.Branch, wt.OriginalBranch)
		if err != nil {
			fmt.Fprintf(out, "\nDivergence:      (error: %v)\n", err)
		} else {
			fmt.Fprintf(out, "\nDivergence:      %d commits ahead, %d commits behind %s\n", ahead, behind, wt.OriginalBranch)
		}
	}

	// Spec files
	if len(wt.SpecFiles) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Spec Files:")
		for _, f := range wt.SpecFiles {
			fmt.Fprintf(out, "  - %s\n", f)
		}
	}

	return nil
}

func runWorktreeRemove(cmd *cobra.Command, args []string) error {
	name := args[0]

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	stateManager := worktree.NewStateManager(wd)
	wt, err := stateManager.FindByName(name)
	if err != nil {
		return fmt.Errorf("failed to find worktree: %w", err)
	}
	if wt == nil {
		return fmt.Errorf("worktree not found: %s", name)
	}

	out := cmd.OutOrStdout()

	// Check for uncommitted changes if worktree exists
	status := getWorktreeStatus(wd, wt)
	if status == "ACTIVE" {
		gitStatus, err := getGitStatus(wt.Path)
		if err == nil && gitStatus != "" && !forceRemove {
			fmt.Fprintln(out, "Warning: worktree has uncommitted changes:")
			fmt.Fprintln(out, gitStatus)
			fmt.Fprintln(out)
			fmt.Fprintln(out, "Use --force to remove anyway.")
			return fmt.Errorf("worktree has uncommitted changes")
		}
	}

	// Confirm unless forced
	if !forceRemove {
		fmt.Fprintf(out, "Remove worktree %q? [y/N] ", name)
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Fprintln(out, "Cancelled.")
			return nil
		}
	}

	// Remove worktree from git (if it exists)
	if status == "ACTIVE" {
		if err := worktree.RemoveWorktree(wd, wt.Path); err != nil {
			fmt.Fprintf(out, "Warning: failed to remove git worktree: %v\n", err)
		}
	}

	// Delete branch
	if err := deleteBranch(wd, wt.Branch); err != nil {
		fmt.Fprintf(out, "Warning: failed to delete branch %s: %v\n", wt.Branch, err)
	}

	// Remove from state
	if err := stateManager.Remove(wt.Path); err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}

	fmt.Fprintf(out, "Removed worktree: %s\n", name)
	return nil
}

func runWorktreeCleanup(cmd *cobra.Command, args []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	stateManager := worktree.NewStateManager(wd)
	worktrees, err := stateManager.List()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	out := cmd.OutOrStdout()

	// Find orphans
	var staleEntries []worktree.WorktreeState
	for _, wt := range worktrees {
		status := getWorktreeStatus(wd, &wt)
		if status != "ACTIVE" {
			staleEntries = append(staleEntries, wt)
		}
	}

	// Find orphan branches (orbital/* branches not in state)
	orphanBranches, err := findOrphanBranches(wd, worktrees)
	if err != nil {
		fmt.Fprintf(out, "Warning: failed to check for orphan branches: %v\n", err)
	}

	// Find orphan git worktrees (not in state)
	orphanGitWorktrees, err := findOrphanGitWorktrees(wd, worktrees)
	if err != nil {
		fmt.Fprintf(out, "Warning: failed to check for orphan git worktrees: %v\n", err)
	}

	if len(staleEntries) == 0 && len(orphanBranches) == 0 && len(orphanGitWorktrees) == 0 {
		fmt.Fprintln(out, "No orphaned worktrees or branches found.")
		return nil
	}

	// Report findings
	if len(staleEntries) > 0 {
		fmt.Fprintln(out, "Stale state entries (worktree path missing):")
		for _, wt := range staleEntries {
			fmt.Fprintf(out, "  - %s (%s)\n", wt.Name, wt.Path)
		}
		fmt.Fprintln(out)
	}

	if len(orphanBranches) > 0 {
		fmt.Fprintln(out, "Orphan orbital/* branches:")
		for _, branch := range orphanBranches {
			fmt.Fprintf(out, "  - %s\n", branch)
		}
		fmt.Fprintln(out)
	}

	if len(orphanGitWorktrees) > 0 {
		fmt.Fprintln(out, "Orphan git worktrees:")
		for _, wt := range orphanGitWorktrees {
			fmt.Fprintf(out, "  - %s\n", wt)
		}
		fmt.Fprintln(out)
	}

	if dryRunCleanup {
		fmt.Fprintln(out, "(dry-run mode - no changes made)")
		return nil
	}

	// Confirm unless forced
	if !forceCleanup {
		fmt.Fprint(out, "Clean up these orphans? [y/N] ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Fprintln(out, "Cancelled.")
			return nil
		}
	}

	// Clean up stale entries
	for _, wt := range staleEntries {
		if err := stateManager.Remove(wt.Path); err != nil {
			fmt.Fprintf(out, "Warning: failed to remove state entry for %s: %v\n", wt.Name, err)
		} else {
			fmt.Fprintf(out, "Removed state entry: %s\n", wt.Name)
		}
		// Also try to delete the branch
		if err := deleteBranch(wd, wt.Branch); err != nil {
			fmt.Fprintf(out, "Warning: failed to delete branch %s: %v\n", wt.Branch, err)
		}
	}

	// Clean up orphan branches
	for _, branch := range orphanBranches {
		if err := deleteBranch(wd, branch); err != nil {
			fmt.Fprintf(out, "Warning: failed to delete branch %s: %v\n", branch, err)
		} else {
			fmt.Fprintf(out, "Deleted branch: %s\n", branch)
		}
	}

	// Clean up orphan git worktrees
	for _, wtPath := range orphanGitWorktrees {
		if err := worktree.RemoveWorktree(wd, wtPath); err != nil {
			fmt.Fprintf(out, "Warning: failed to remove git worktree %s: %v\n", wtPath, err)
		} else {
			fmt.Fprintf(out, "Removed git worktree: %s\n", wtPath)
		}
	}

	fmt.Fprintln(out, "Cleanup complete.")
	return nil
}

// getWorktreeStatus checks the status of a worktree.
func getWorktreeStatus(repoDir string, wt *worktree.WorktreeState) string {
	// Check if path exists
	info, err := os.Stat(wt.Path)
	if os.IsNotExist(err) {
		return "MISSING"
	}
	if err != nil || !info.IsDir() {
		return "MISSING"
	}

	// Check if it's registered as a git worktree
	if !isGitWorktree(repoDir, wt.Path) {
		return "ORPHAN"
	}

	return "ACTIVE"
}

// isGitWorktree checks if a path is registered as a git worktree.
func isGitWorktree(repoDir, wtPath string) bool {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Parse worktree list output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimPrefix(line, "worktree ")
			if path == wtPath {
				return true
			}
		}
	}
	return false
}

// getGitStatus returns the git status output for a path.
func getGitStatus(path string) (string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = path
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getBranchDivergence returns how many commits a branch is ahead/behind another.
func getBranchDivergence(repoDir, branch, baseBranch string) (ahead, behind int, err error) {
	// Get ahead count
	aheadCmd := exec.Command("git", "rev-list", "--count", fmt.Sprintf("%s..%s", baseBranch, branch))
	aheadCmd.Dir = repoDir
	aheadOutput, err := aheadCmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get ahead count: %w", err)
	}
	fmt.Sscanf(strings.TrimSpace(string(aheadOutput)), "%d", &ahead)

	// Get behind count
	behindCmd := exec.Command("git", "rev-list", "--count", fmt.Sprintf("%s..%s", branch, baseBranch))
	behindCmd.Dir = repoDir
	behindOutput, err := behindCmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get behind count: %w", err)
	}
	fmt.Sscanf(strings.TrimSpace(string(behindOutput)), "%d", &behind)

	return ahead, behind, nil
}

// deleteBranch deletes a git branch.
func deleteBranch(repoDir, branch string) error {
	cmd := exec.Command("git", "branch", "-D", branch)
	cmd.Dir = repoDir
	return cmd.Run()
}

// findOrphanBranches finds orbital/* branches not tracked in state.
func findOrphanBranches(repoDir string, trackedWorktrees []worktree.WorktreeState) ([]string, error) {
	cmd := exec.Command("git", "branch", "--list", "orbital/*")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Build set of tracked branches
	tracked := make(map[string]bool)
	for _, wt := range trackedWorktrees {
		tracked[wt.Branch] = true
	}

	// Find orphans
	var orphans []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		branch := strings.TrimSpace(line)
		branch = strings.TrimPrefix(branch, "* ") // Remove current branch marker
		if branch == "" {
			continue
		}
		if !tracked[branch] {
			orphans = append(orphans, branch)
		}
	}

	return orphans, nil
}

// findOrphanGitWorktrees finds git worktrees under .orbital/worktrees not in state.
func findOrphanGitWorktrees(repoDir string, trackedWorktrees []worktree.WorktreeState) ([]string, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoDir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Build set of tracked paths
	tracked := make(map[string]bool)
	for _, wt := range trackedWorktrees {
		tracked[wt.Path] = true
	}

	// Find orphans (only in .orbital/worktrees/)
	var orphans []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimPrefix(line, "worktree ")
			if strings.Contains(path, ".orbital/worktrees/") && !tracked[path] {
				orphans = append(orphans, path)
			}
		}
	}

	return orphans, nil
}
