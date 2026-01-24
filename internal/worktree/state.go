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
	Path           string    `json:"path"`
	Branch         string    `json:"branch"`
	OriginalBranch string    `json:"originalBranch"`
	SpecFiles      []string  `json:"specFiles"`
	SessionID      string    `json:"sessionId"`
	CreatedAt      time.Time `json:"createdAt"`
}

// StateFile represents the persisted state of all worktrees.
type StateFile struct {
	Worktrees []WorktreeState `json:"worktrees"`
}

// StateManager handles worktree state persistence.
type StateManager struct {
	stateDir string
}

// NewStateManager creates a new StateManager.
func NewStateManager(workingDir string) *StateManager {
	return &StateManager{
		stateDir: filepath.Join(workingDir, ".orbit"),
	}
}

// StatePath returns the path to the state file.
func (m *StateManager) StatePath() string {
	return filepath.Join(m.stateDir, "worktree-state.json")
}

// Load loads the worktree state from disk.
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
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &state, nil
}

// Save persists the worktree state to disk.
func (m *StateManager) Save(state *StateFile) error {
	// Ensure .orbit directory exists
	if err := os.MkdirAll(m.stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(m.StatePath(), data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// Add adds a new worktree to the state.
func (m *StateManager) Add(wt WorktreeState) error {
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

// Remove removes a worktree from the state by path.
func (m *StateManager) Remove(path string) error {
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

// UpdateSessionID updates the Claude session ID for a worktree.
func (m *StateManager) UpdateSessionID(path, sessionID string) error {
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
