package worktree

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WorktreeState represents the persisted state of a single worktree.
type WorktreeState struct {
	// Name is the human-readable name of the worktree (e.g., "swift-falcon").
	// Added in v2 - may be empty for worktrees created before this field existed.
	Name           string    `json:"name,omitempty"`
	Path           string    `json:"path"`
	Branch         string    `json:"branch"`
	OriginalBranch string    `json:"originalBranch"`
	SpecFiles      []string  `json:"specFiles"`
	SessionID      string    `json:"sessionId,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

// StateFile represents the persisted state of all worktrees.
type StateFile struct {
	Worktrees []WorktreeState `json:"worktrees"`
}

// StateManager handles worktree state persistence.
type StateManager struct {
	stateDir   string
	workingDir string
}

// Lock timeout and stale lock detection constants.
const (
	lockTimeout  = 5 * time.Second
	staleLockAge = 30 * time.Second
)

// NewStateManager creates a new StateManager.
func NewStateManager(workingDir string) *StateManager {
	return &StateManager{
		stateDir:   filepath.Join(workingDir, ".orbital"),
		workingDir: workingDir,
	}
}

// StatePath returns the path to the state file.
func (m *StateManager) StatePath() string {
	return filepath.Join(m.stateDir, "worktree-state.json")
}

// lockPath returns the path to the lock file.
func (m *StateManager) lockPath() string {
	return m.StatePath() + ".lock"
}

// backupPath returns the path to the backup file.
func (m *StateManager) backupPath() string {
	return m.StatePath() + ".bak"
}

// acquireLock attempts to acquire an exclusive lock on the state file.
// Returns a release function that must be called when done.
func (m *StateManager) acquireLock() (func(), error) {
	// Ensure .orbital directory exists before trying to create lock file
	if err := os.MkdirAll(m.stateDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory for lock: %w", err)
	}

	lockFile := m.lockPath()
	deadline := time.Now().Add(lockTimeout)

	for time.Now().Before(deadline) {
		// Try to create lock file exclusively
		f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			// Write our PID to the lock file for debugging
			fmt.Fprintf(f, "%d\n", os.Getpid())
			f.Close()

			// Return release function
			return func() {
				os.Remove(lockFile)
			}, nil
		}

		// Check if lock is stale
		if info, statErr := os.Stat(lockFile); statErr == nil {
			if time.Since(info.ModTime()) > staleLockAge {
				// Lock is stale, remove it and retry
				os.Remove(lockFile)
				continue
			}
		}

		// Wait a bit before retrying
		time.Sleep(100 * time.Millisecond)
	}

	return nil, fmt.Errorf("failed to acquire state lock after %v (lock file: %s)", lockTimeout, lockFile)
}

// Load loads the worktree state from disk.
// If the state file is corrupted, it attempts recovery from backup.
func (m *StateManager) Load() (*StateFile, error) {
	data, err := os.ReadFile(m.StatePath())
	if os.IsNotExist(err) {
		return &StateFile{Worktrees: []WorktreeState{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state StateFile
	if err := json.Unmarshal(data, &state); err != nil {
		// Try to recover from backup
		recovered, recoverErr := m.recoverFromBackup()
		if recoverErr == nil {
			return recovered, nil
		}
		return nil, fmt.Errorf("failed to parse state file (recovery failed: %v): %w", recoverErr, err)
	}

	// Migrate any relative paths to absolute paths
	m.migratePaths(&state)

	return &state, nil
}

// recoverFromBackup attempts to recover state from the backup file.
func (m *StateManager) recoverFromBackup() (*StateFile, error) {
	backupData, err := os.ReadFile(m.backupPath())
	if err != nil {
		return nil, fmt.Errorf("no backup file available: %w", err)
	}

	var state StateFile
	if err := json.Unmarshal(backupData, &state); err != nil {
		return nil, fmt.Errorf("backup file also corrupted: %w", err)
	}

	// Restore from backup
	if err := m.atomicWrite(&state); err != nil {
		return nil, fmt.Errorf("failed to restore from backup: %w", err)
	}

	return &state, nil
}

// migratePaths converts any relative paths to absolute paths for backwards compatibility.
func (m *StateManager) migratePaths(state *StateFile) {
	for i := range state.Worktrees {
		if state.Worktrees[i].Path != "" && !filepath.IsAbs(state.Worktrees[i].Path) {
			absPath, err := filepath.Abs(filepath.Join(m.workingDir, state.Worktrees[i].Path))
			if err == nil {
				state.Worktrees[i].Path = absPath
			}
		}
	}
}

// Save persists the worktree state to disk atomically.
func (m *StateManager) Save(state *StateFile) error {
	return m.atomicWrite(state)
}

// atomicWrite writes the state file atomically using temp file + rename.
func (m *StateManager) atomicWrite(state *StateFile) error {
	// Ensure .orbital directory exists
	if err := os.MkdirAll(m.stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Create backup of existing file if it exists
	if _, err := os.Stat(m.StatePath()); err == nil {
		// Copy current to backup (ignore errors - backup is best effort)
		if existingData, readErr := os.ReadFile(m.StatePath()); readErr == nil {
			_ = os.WriteFile(m.backupPath(), existingData, 0644)
		}
	}

	// Write to temp file in same directory (same filesystem for atomic rename)
	tmpFile, err := os.CreateTemp(m.stateDir, "worktree-state-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Clean up temp file on error
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Sync to disk before rename to ensure durability
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}
	tmpFile.Close()

	// Atomic rename (POSIX guarantees atomicity on same filesystem)
	if err := os.Rename(tmpPath, m.StatePath()); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	success = true
	return nil
}

// Add adds a new worktree to the state with locking.
func (m *StateManager) Add(wt WorktreeState) error {
	// Validate path is absolute
	if wt.Path != "" && !filepath.IsAbs(wt.Path) {
		return fmt.Errorf("worktree path must be absolute, got relative path: %s", wt.Path)
	}

	// Acquire lock for thread-safe modification
	release, err := m.acquireLock()
	if err != nil {
		return err
	}
	defer release()

	state, err := m.Load()
	if err != nil {
		return err
	}

	// Set creation time if not already set
	if wt.CreatedAt.IsZero() {
		wt.CreatedAt = time.Now()
	}

	state.Worktrees = append(state.Worktrees, wt)
	return m.Save(state)
}

// Remove removes a worktree from the state by path with locking.
func (m *StateManager) Remove(path string) error {
	// Acquire lock for thread-safe modification
	release, err := m.acquireLock()
	if err != nil {
		return err
	}
	defer release()

	state, err := m.Load()
	if err != nil {
		return err
	}

	var filtered []WorktreeState
	for _, wt := range state.Worktrees {
		if wt.Path != path {
			filtered = append(filtered, wt)
		}
	}

	state.Worktrees = filtered
	return m.Save(state)
}

// FindBySpecFile finds worktrees associated with a given spec file.
func (m *StateManager) FindBySpecFile(specFile string) ([]WorktreeState, error) {
	state, err := m.Load()
	if err != nil {
		return nil, err
	}

	var matches []WorktreeState
	for _, wt := range state.Worktrees {
		for _, sf := range wt.SpecFiles {
			if sf == specFile {
				matches = append(matches, wt)
				break
			}
		}
	}

	return matches, nil
}

// List returns all tracked worktrees.
func (m *StateManager) List() ([]WorktreeState, error) {
	state, err := m.Load()
	if err != nil {
		return nil, err
	}
	return state.Worktrees, nil
}

// FindByPath finds a worktree by its path.
func (m *StateManager) FindByPath(path string) (*WorktreeState, error) {
	state, err := m.Load()
	if err != nil {
		return nil, err
	}

	for _, wt := range state.Worktrees {
		if wt.Path == path {
			return &wt, nil
		}
	}

	return nil, nil
}

// FindByName finds a worktree by its name.
func (m *StateManager) FindByName(name string) (*WorktreeState, error) {
	state, err := m.Load()
	if err != nil {
		return nil, err
	}

	for _, wt := range state.Worktrees {
		if wt.Name == name {
			return &wt, nil
		}
	}

	return nil, nil
}

// UpdateSessionID updates the Claude session ID for a worktree with locking.
func (m *StateManager) UpdateSessionID(path, sessionID string) error {
	// Acquire lock for thread-safe modification
	release, err := m.acquireLock()
	if err != nil {
		return err
	}
	defer release()

	state, err := m.Load()
	if err != nil {
		return err
	}

	for i := range state.Worktrees {
		if state.Worktrees[i].Path == path {
			state.Worktrees[i].SessionID = sessionID
			return m.Save(state)
		}
	}

	return fmt.Errorf("worktree not found: %s", path)
}

// ValidateWorktree checks if a worktree path exists and is valid.
func ValidateWorktree(wt *WorktreeState) error {
	// Check path exists
	info, err := os.Stat(wt.Path)
	if os.IsNotExist(err) {
		return fmt.Errorf("worktree directory not found: %s\nRun 'orbital worktree cleanup' to remove stale entries", wt.Path)
	}
	if err != nil {
		return fmt.Errorf("failed to check worktree path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("worktree path is not a directory: %s", wt.Path)
	}

	// Check it's a git worktree (has .git file, not directory)
	gitPath := filepath.Join(wt.Path, ".git")
	gitInfo, err := os.Stat(gitPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("not a git worktree (missing .git): %s", wt.Path)
	}
	if err != nil {
		return fmt.Errorf("failed to check .git path: %w", err)
	}
	// Git worktrees have a .git file (not directory) pointing to the main repo
	if gitInfo.IsDir() {
		return fmt.Errorf("path is a git repository, not a worktree (.git is directory): %s", wt.Path)
	}

	return nil
}
