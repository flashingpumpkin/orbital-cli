package worktree

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// BranchPrefix is the prefix used for worktree branches.
const BranchPrefix = "orbital/"

// WorktreeDir is the directory under .orbital where worktrees are created.
const WorktreeDir = ".orbital/worktrees"

// ValidateBranchName validates that a branch name matches the expected format.
// Branch names must start with "orbital/", contain no spaces, and only have
// alphanumeric characters, hyphens, and slashes.
func ValidateBranchName(name string) error {
	if !strings.HasPrefix(name, BranchPrefix) {
		return fmt.Errorf("branch name %q must start with %q", name, BranchPrefix)
	}

	// Check for spaces
	if strings.Contains(name, " ") {
		return fmt.Errorf("branch name %q contains spaces", name)
	}

	// Check for invalid characters (only allow alphanumeric, hyphen, slash)
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9/-]+$`)
	if !validPattern.MatchString(name) {
		return fmt.Errorf("branch name %q contains invalid characters (only alphanumeric, hyphen, and slash allowed)", name)
	}

	// Check for common corruption patterns (e.g., "success" appended)
	corruptionPatterns := []string{"success", "failure", "error", "true", "false"}
	suffix := strings.TrimPrefix(name, BranchPrefix)
	for _, pattern := range corruptionPatterns {
		if strings.HasSuffix(strings.ToLower(suffix), pattern) && !strings.HasSuffix(suffix, "-"+pattern) {
			return fmt.Errorf("branch name %q appears corrupted (contains %q without hyphen separator)", name, pattern)
		}
	}

	return nil
}

// ValidateWorktreeName validates that a worktree name is valid.
// Names must be lowercase, contain only alphanumeric characters and hyphens,
// and not start or end with a hyphen.
func ValidateWorktreeName(name string) error {
	if name == "" {
		return fmt.Errorf("worktree name cannot be empty")
	}

	// Check for valid characters (lowercase alphanumeric and hyphens)
	validPattern := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	if !validPattern.MatchString(name) {
		return fmt.Errorf("worktree name %q must be lowercase with hyphens (e.g., swift-falcon)", name)
	}

	return nil
}

// CreateWorktree creates a new git worktree with the given name.
// The worktree is created at .orbital/worktrees/<name> with branch orbital/<name>.
func CreateWorktree(dir, name string) error {
	if err := ValidateWorktreeName(name); err != nil {
		return err
	}

	worktreePath := filepath.Join(WorktreeDir, name)
	branchName := BranchPrefix + name

	cmd := exec.Command("git", "-C", dir, "worktree", "add", "-b", branchName, worktreePath, "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create worktree %q at %q: %w\ngit output: %s",
			name, worktreePath, err, strings.TrimSpace(string(output)))
	}

	return nil
}

// RemoveWorktree removes a git worktree at the specified path.
func RemoveWorktree(dir, worktreePath string) error {
	if worktreePath == "" {
		return fmt.Errorf("worktree path cannot be empty")
	}

	cmd := exec.Command("git", "-C", dir, "worktree", "remove", worktreePath, "--force")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove worktree %q: %w\ngit output: %s",
			worktreePath, err, strings.TrimSpace(string(output)))
	}

	return nil
}

// DeleteBranch deletes a git branch with the given name.
// It first attempts a safe delete (-d), and falls back to force delete (-D) if needed.
func DeleteBranch(dir, branchName string) error {
	if err := ValidateBranchName(branchName); err != nil {
		return err
	}

	// Try safe delete first
	cmd := exec.Command("git", "-C", dir, "branch", "-d", branchName)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}

	// Safe delete failed, try force delete
	forceCmd := exec.Command("git", "-C", dir, "branch", "-D", branchName)
	forceOutput, forceErr := forceCmd.CombinedOutput()
	if forceErr != nil {
		return fmt.Errorf("failed to delete branch %q: %w\ngit branch -d output: %s\ngit branch -D output: %s",
			branchName, forceErr, strings.TrimSpace(string(output)), strings.TrimSpace(string(forceOutput)))
	}

	return nil
}

// ListWorktreeNames returns the names of existing worktrees by reading the state file.
// This is used for collision avoidance when generating new names.
func ListWorktreeNames(dir string) ([]string, error) {
	manager := NewStateManager(dir)
	state, err := manager.Load()
	if err != nil {
		return nil, err
	}

	var names []string
	for _, wt := range state.Worktrees {
		names = append(names, wt.Name)
	}
	return names, nil
}

// WorktreePath returns the full path for a worktree with the given name.
func WorktreePath(name string) string {
	return filepath.Join(WorktreeDir, name)
}

// BranchName returns the branch name for a worktree with the given name.
func BranchName(name string) string {
	return BranchPrefix + name
}
