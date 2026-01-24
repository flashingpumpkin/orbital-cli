package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Registry manages all sessions for a project.
type Registry struct {
	mu         sync.RWMutex
	sessions   map[string]*Session
	projectDir string
	stateFile  string
}

// NewRegistry creates a new session registry.
func NewRegistry(projectDir string) *Registry {
	return &Registry{
		sessions:   make(map[string]*Session),
		projectDir: projectDir,
		stateFile:  filepath.Join(projectDir, ".orbital", "daemon-state.json"),
	}
}

// Add registers a new session.
func (r *Registry) Add(session *Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.sessions[session.ID]; exists {
		return fmt.Errorf("session %s already exists", session.ID)
	}

	// Initialize runtime fields
	session.outputBuffer = NewRingBuffer(10000)
	session.subscribers = make([]chan OutputMsg, 0)
	session.done = make(chan struct{})

	r.sessions[session.ID] = session
	return r.saveLocked()
}

// Get retrieves a session by ID.
// Returns a clone of the session for thread-safe access.
func (r *Registry) Get(id string) (*Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	session, ok := r.sessions[id]
	if !ok {
		return nil, false
	}
	return session.Clone(), true
}

// GetInternal retrieves the actual session pointer for internal use.
// Caller must ensure proper synchronization.
func (r *Registry) GetInternal(id string) (*Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	session, ok := r.sessions[id]
	return session, ok
}

// Remove deletes a session from the registry.
func (r *Registry) Remove(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.sessions, id)
	return r.saveLocked()
}

// List returns clones of all sessions for thread-safe access.
func (r *Registry) List() []*Session {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		result = append(result, s.Clone())
	}
	return result
}

// ListByStatus returns clones of sessions filtered by status.
func (r *Registry) ListByStatus(status SessionStatus) []*Session {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Session
	for _, s := range r.sessions {
		if s.Status == status {
			result = append(result, s.Clone())
		}
	}
	return result
}

// Count returns session counts by status.
func (r *Registry) Count() SessionCounts {
	r.mu.RLock()
	defer r.mu.RUnlock()

	counts := SessionCounts{}
	for _, s := range r.sessions {
		counts.Total++
		switch s.Status {
		case StatusRunning:
			counts.Running++
		case StatusCompleted, StatusMerged:
			counts.Completed++
		case StatusFailed, StatusConflict:
			counts.Failed++
		case StatusStopped, StatusInterrupted:
			counts.Stopped++
		}
	}
	return counts
}

// TotalCost returns the sum of all session costs.
func (r *Registry) TotalCost() float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var total float64
	for _, s := range r.sessions {
		total += s.TotalCost
	}
	return total
}

// Update updates a session and persists the change.
func (r *Registry) Update(session *Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.sessions[session.ID]; !exists {
		return fmt.Errorf("session %s not found", session.ID)
	}

	r.sessions[session.ID] = session
	return r.saveLocked()
}

// UpdateStatus updates a session's status and optionally sets completion time.
func (r *Registry) UpdateStatus(id string, status SessionStatus, errMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, exists := r.sessions[id]
	if !exists {
		return fmt.Errorf("session %s not found", id)
	}

	session.Status = status
	if errMsg != "" {
		session.Error = errMsg
	}

	// Set completion time and close done channel for terminal states
	switch status {
	case StatusCompleted, StatusFailed, StatusStopped, StatusMerged, StatusConflict, StatusInterrupted:
		now := time.Now()
		session.CompletedAt = &now
		// Close done channel to notify SSE subscribers (safe to call multiple times)
		session.CloseDone()
	}

	return r.saveLocked()
}

// UpdateProgress updates a session's iteration and cost.
func (r *Registry) UpdateProgress(id string, iteration int, cost float64, tokensIn, tokensOut int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, exists := r.sessions[id]
	if !exists {
		return fmt.Errorf("session %s not found", id)
	}

	session.Iteration = iteration
	session.TotalCost = cost
	session.TokensIn = tokensIn
	session.TokensOut = tokensOut

	return r.saveLocked()
}

// Load reads the registry state from disk.
func (r *Registry) Load() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for temp file left over from interrupted write
	tempPath := r.stateFile + ".tmp"
	if _, err := os.Stat(tempPath); err == nil {
		// Temp file exists - try to use it if main file is missing/corrupt
		os.Remove(tempPath) // Clean up orphaned temp file
	}

	// Check for backup file and recover if main file is corrupt
	backupPath := r.stateFile + ".bak"

	data, err := os.ReadFile(r.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Try to recover from backup
			data, err = os.ReadFile(backupPath)
			if err != nil {
				if os.IsNotExist(err) {
					// No state file yet, that's fine
					return nil
				}
				return fmt.Errorf("failed to read backup state file: %w", err)
			}
			// Recovered from backup
		} else {
			return fmt.Errorf("failed to read state file: %w", err)
		}
	}

	var state struct {
		Sessions map[string]*Session `json:"sessions"`
	}

	if err := json.Unmarshal(data, &state); err != nil {
		// Main file is corrupt, try backup
		backupData, backupErr := os.ReadFile(backupPath)
		if backupErr != nil {
			return fmt.Errorf("failed to unmarshal state (and backup not available): %w", err)
		}
		if unmarshalErr := json.Unmarshal(backupData, &state); unmarshalErr != nil {
			return fmt.Errorf("failed to unmarshal state and backup: main=%v, backup=%v", err, unmarshalErr)
		}
		// Successfully recovered from backup
	}

	// Initialize runtime fields and mark running sessions as interrupted
	for id, session := range state.Sessions {
		session.outputBuffer = NewRingBuffer(10000)
		session.subscribers = make([]chan OutputMsg, 0)
		session.done = make(chan struct{})

		// Mark previously running sessions as interrupted
		if session.Status == StatusRunning || session.Status == StatusMerging {
			session.Status = StatusInterrupted
		}

		// Close done channel for already-terminal sessions
		switch session.Status {
		case StatusCompleted, StatusFailed, StatusStopped, StatusMerged, StatusConflict, StatusInterrupted:
			session.CloseDone()
		}

		r.sessions[id] = session
	}

	return nil
}

// saveLocked persists the registry state to disk.
// Caller must hold the write lock.
func (r *Registry) saveLocked() error {
	stateDir := filepath.Dir(r.stateFile)
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Clone sessions to avoid data races during serialization
	// (sessions may be modified by runners while we serialize)
	clonedSessions := make(map[string]*Session, len(r.sessions))
	for id, session := range r.sessions {
		clonedSessions[id] = session.Clone()
	}

	state := struct {
		Sessions map[string]*Session `json:"sessions"`
	}{
		Sessions: clonedSessions,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Create backup of existing state file before writing
	backupPath := r.stateFile + ".bak"
	if _, err := os.Stat(r.stateFile); err == nil {
		// Main file exists, create backup
		if backupErr := copyFile(r.stateFile, backupPath); backupErr != nil {
			// Log but don't fail - backup is best-effort
		}
	}

	// Atomic write
	tempPath := r.stateFile + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp state file: %w", err)
	}

	if err := os.Rename(tempPath, r.stateFile); err != nil {
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// Subscribe adds a subscriber to a session's output stream.
// Returns the message channel, output history, done channel (closed on session end), and error.
func (r *Registry) Subscribe(id string) (chan OutputMsg, []OutputMsg, <-chan struct{}, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, exists := r.sessions[id]
	if !exists {
		return nil, nil, nil, fmt.Errorf("session %s not found", id)
	}

	// Lock session for safe access to subscribers
	session.mu.Lock()
	defer session.mu.Unlock()

	// Get existing output
	history := session.outputBuffer.ReadAll()

	// Create subscriber channel
	ch := make(chan OutputMsg, 100)
	session.subscribers = append(session.subscribers, ch)

	return ch, history, session.done, nil
}

// Unsubscribe removes a subscriber from a session's output stream.
func (r *Registry) Unsubscribe(id string, ch chan OutputMsg) {
	r.mu.RLock()
	session, exists := r.sessions[id]
	r.mu.RUnlock()

	if !exists {
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	for i, sub := range session.subscribers {
		if sub == ch {
			session.subscribers = append(session.subscribers[:i], session.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

// Broadcast sends a message to all subscribers of a session.
func (r *Registry) Broadcast(id string, msg OutputMsg) {
	r.mu.RLock()
	session, exists := r.sessions[id]
	r.mu.RUnlock()

	if !exists {
		return
	}

	// Add to buffer (RingBuffer has its own lock)
	session.outputBuffer.Write(msg)

	// Lock session for safe access to subscribers
	session.mu.RLock()
	subscribers := make([]chan OutputMsg, len(session.subscribers))
	copy(subscribers, session.subscribers)
	session.mu.RUnlock()

	// Send to all subscribers (non-blocking)
	for _, ch := range subscribers {
		select {
		case ch <- msg:
		default:
			// Channel full, skip
		}
	}
}
