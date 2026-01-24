// Package state provides session state management for orbit.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/flashingpumpkin/orbital/internal/workflow"
)

// WorkflowState captures the workflow configuration and progress.
type WorkflowState struct {
	// PresetName is the name of the preset used, if any.
	PresetName string `json:"preset_name,omitempty"`

	// Steps contains the full workflow step definitions.
	Steps []workflow.Step `json:"steps"`

	// CurrentStepIndex is the index of the current step being executed.
	CurrentStepIndex int `json:"current_step_index"`

	// GateRetries tracks the number of times each gate step has failed.
	GateRetries map[string]int `json:"gate_retries,omitempty"`
}

// State represents the current execution state of a orbit session.
type State struct {
	SessionID    string    `json:"session_id"`
	PID          int       `json:"pid"`
	WorkingDir   string    `json:"working_dir"`
	ActiveFiles  []string  `json:"active_files"`
	StartedAt    time.Time `json:"started_at"`
	Iteration    int       `json:"iteration"`
	TotalCost    float64   `json:"total_cost"`
	NotesFile    string    `json:"notes_file,omitempty"`
	ContextFiles []string  `json:"context_files,omitempty"`

	// Workflow captures the workflow configuration and progress.
	Workflow *WorkflowState `json:"workflow,omitempty"`
}

// StateDir returns the path to the state directory for the given working directory.
func StateDir(workingDir string) string {
	workingDir = strings.TrimSuffix(workingDir, "/")
	return filepath.Join(workingDir, ".orbital", "state")
}

// NewState creates a new State with the current process ID and timestamp.
func NewState(sessionID string, workingDir string, files []string, notesFile string, contextFiles []string) *State {
	return &State{
		SessionID:    sessionID,
		PID:          os.Getpid(),
		WorkingDir:   workingDir,
		ActiveFiles:  files,
		StartedAt:    time.Now(),
		Iteration:    0,
		TotalCost:    0.0,
		NotesFile:    notesFile,
		ContextFiles: contextFiles,
	}
}

// Save persists the state to state.json in the state directory.
func (s *State) Save() error {
	stateDir := StateDir(s.WorkingDir)

	// Create state directory if it doesn't exist
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	statePath := filepath.Join(stateDir, "state.json")

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write to temp file and rename for atomicity
	tempPath := statePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	if err := os.Rename(tempPath, statePath); err != nil {
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	return nil
}

// Load reads the state from state.json in the state directory.
func Load(workingDir string) (*State, error) {
	stateDir := StateDir(workingDir)
	statePath := filepath.Join(stateDir, "state.json")

	data, err := os.ReadFile(statePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &s, nil
}

// Exists returns true if a state file exists in the working directory.
func Exists(workingDir string) bool {
	stateDir := StateDir(workingDir)
	statePath := filepath.Join(stateDir, "state.json")
	_, err := os.Stat(statePath)
	return err == nil
}

// IsStale returns true if the process with the stored PID is no longer running.
func (s *State) IsStale() bool {
	// Send signal 0 to check if process exists
	err := syscall.Kill(s.PID, 0)
	// If no error, process exists; if error, it's stale
	return err != nil
}

// Cleanup removes the state directory and its contents.
func (s *State) Cleanup() error {
	stateDir := StateDir(s.WorkingDir)
	if err := os.RemoveAll(stateDir); err != nil {
		return fmt.Errorf("failed to remove state directory: %w", err)
	}
	return nil
}

// UpdateIteration updates the iteration count and total cost.
func (s *State) UpdateIteration(iteration int, cost float64) {
	s.Iteration = iteration
	s.TotalCost = cost
}

// SetWorkflow initialises the workflow state from a workflow configuration.
func (s *State) SetWorkflow(w *workflow.Workflow) {
	s.Workflow = &WorkflowState{
		PresetName:       w.Preset,
		Steps:            w.Steps,
		CurrentStepIndex: 0,
		GateRetries:      make(map[string]int),
	}
}

// UpdateWorkflowStep updates the current step index.
func (s *State) UpdateWorkflowStep(stepIndex int) {
	if s.Workflow != nil {
		s.Workflow.CurrentStepIndex = stepIndex
	}
}

// IncrementGateRetry increments the retry count for a gate step.
func (s *State) IncrementGateRetry(stepName string) {
	if s.Workflow != nil {
		if s.Workflow.GateRetries == nil {
			s.Workflow.GateRetries = make(map[string]int)
		}
		s.Workflow.GateRetries[stepName]++
	}
}

// GetGateRetryCount returns the current retry count for a gate step.
func (s *State) GetGateRetryCount(stepName string) int {
	if s.Workflow == nil || s.Workflow.GateRetries == nil {
		return 0
	}
	return s.Workflow.GateRetries[stepName]
}
