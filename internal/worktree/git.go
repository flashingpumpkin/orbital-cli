package worktree

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// gitCommandTimeout is the maximum time a git command can run before being killed.
const gitCommandTimeout = 30 * time.Second

// runGitCommand runs a git command with a timeout.
// Returns the combined stdout/stderr output and any error.
func runGitCommand(dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return output, fmt.Errorf("git command timed out after %v: git %s", gitCommandTimeout, strings.Join(args, " "))
	}
	return output, err
}

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

	output, err := runGitCommand(dir, "worktree", "add", "-b", branchName, worktreePath, "HEAD")
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

	output, err := runGitCommand(dir, "worktree", "remove", worktreePath, "--force")
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
	output, err := runGitCommand(dir, "branch", "-d", branchName)
	if err == nil {
		return nil
	}

	// Safe delete failed, try force delete
	forceOutput, forceErr := runGitCommand(dir, "branch", "-D", branchName)
	if forceErr != nil {
		return fmt.Errorf("failed to delete branch %q: %w\ngit branch -d output: %s\ngit branch -D output: %s",
			branchName, forceErr, strings.TrimSpace(string(output)), strings.TrimSpace(string(forceOutput)))
	}

	return nil
}

// ListWorktreeNames returns the names of existing worktrees.
// It combines names from both the state file and actual git worktrees to ensure
// collision avoidance even when state is out of sync (e.g., after a crash).
func ListWorktreeNames(dir string) ([]string, error) {
	nameSet := make(map[string]bool)

	// Get names from state file
	manager := NewStateManager(dir)
	state, err := manager.Load()
	if err == nil {
		for _, wt := range state.Worktrees {
			if wt.Name != "" {
				nameSet[wt.Name] = true
			}
		}
	}

	// Get names from actual git worktrees
	gitNames, err := listGitWorktreeNames(dir)
	if err == nil {
		for _, name := range gitNames {
			nameSet[name] = true
		}
	}

	// Convert set to slice
	var names []string
	for name := range nameSet {
		names = append(names, name)
	}
	return names, nil
}

// listGitWorktreeNames queries git for existing worktrees and extracts names
// for any that match the orbital worktree pattern (.orbital/worktrees/<name>).
func listGitWorktreeNames(dir string) ([]string, error) {
	output, err := runGitCommand(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	var names []string
	worktreePathPrefix := filepath.Join(WorktreeDir) + string(filepath.Separator)

	// Parse porcelain output: each worktree starts with "worktree <path>"
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimPrefix(line, "worktree ")
			// Check if this is an orbital worktree
			if idx := strings.Index(path, worktreePathPrefix); idx != -1 {
				// Extract the name (everything after .orbital/worktrees/)
				namePart := path[idx+len(worktreePathPrefix):]
				// Handle any trailing path components
				if sepIdx := strings.IndexAny(namePart, "/\\"); sepIdx != -1 {
					namePart = namePart[:sepIdx]
				}
				if namePart != "" {
					names = append(names, namePart)
				}
			}
		}
	}

	return names, nil
}

// ListGitBranches returns all branches matching the orbital prefix.
func ListGitBranches(dir string) ([]string, error) {
	output, err := runGitCommand(dir, "branch", "--list", BranchPrefix+"*", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}

	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// VerifyBranchExists checks if a branch exists in the repository.
func VerifyBranchExists(dir, branchName string) error {
	output, err := runGitCommand(dir, "rev-parse", "--verify", branchName)
	if err != nil {
		return fmt.Errorf("branch %q does not exist: %w\ngit output: %s", branchName, err, strings.TrimSpace(string(output)))
	}
	return nil
}

// VerifyOnBranch checks if the repository is currently on the specified branch.
func VerifyOnBranch(dir, expectedBranch string) error {
	output, err := runGitCommand(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	currentBranch := strings.TrimSpace(string(output))
	if currentBranch != expectedBranch {
		return fmt.Errorf("expected to be on branch %q but currently on %q", expectedBranch, currentBranch)
	}
	return nil
}

// VerifyBranchContains checks if a branch contains a specific commit (is ancestor of).
func VerifyBranchContains(dir, branch, commit string) error {
	_, err := runGitCommand(dir, "merge-base", "--is-ancestor", commit, branch)
	if err != nil {
		return fmt.Errorf("branch %q does not contain commit %q", branch, commit)
	}
	return nil
}

// GetBranchHeadCommit returns the commit SHA of the branch HEAD.
func GetBranchHeadCommit(dir, branch string) (string, error) {
	output, err := runGitCommand(dir, "rev-parse", branch)
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD of branch %q: %w", branch, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// WorktreePath returns the full path for a worktree with the given name.
func WorktreePath(name string) string {
	return filepath.Join(WorktreeDir, name)
}

// BranchName returns the branch name for a worktree with the given name.
func BranchName(name string) string {
	return BranchPrefix + name
}
