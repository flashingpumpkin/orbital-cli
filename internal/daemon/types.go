// Package daemon provides the orbital daemon for managing multiple concurrent sessions.
package daemon

import (
	"sync"
	"time"

	"github.com/flashingpumpkin/orbital/internal/workflow"
)

// SessionStatus represents the current state of a session.
type SessionStatus string

const (
	StatusRunning     SessionStatus = "running"
	StatusCompleted   SessionStatus = "completed"
	StatusFailed      SessionStatus = "failed"
	StatusStopped     SessionStatus = "stopped"
	StatusMerging     SessionStatus = "merging"
	StatusMerged      SessionStatus = "merged"
	StatusInterrupted SessionStatus = "interrupted"
	StatusConflict    SessionStatus = "conflict"
)

// WorktreeInfo contains worktree details for a session.
type WorktreeInfo struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	Branch         string `json:"branch"`
	OriginalBranch string `json:"original_branch"`
}

// WorkflowState captures workflow progress.
type WorkflowState struct {
	PresetName       string          `json:"preset_name,omitempty"`
	Steps            []workflow.Step `json:"steps"`
	CurrentStepIndex int             `json:"current_step_index"`
	GateRetries      map[string]int  `json:"gate_retries,omitempty"`
}

// Session represents a running or completed orbital session.
type Session struct {
	ID             string         `json:"id"`
	SpecFiles      []string       `json:"spec_files"`
	Status         SessionStatus  `json:"status"`
	WorkingDir     string         `json:"working_dir"`
	Worktree       *WorktreeInfo  `json:"worktree,omitempty"`
	Iteration      int            `json:"iteration"`
	MaxIterations  int            `json:"max_iterations"`
	TotalCost      float64        `json:"total_cost"`
	MaxBudget      float64        `json:"max_budget"`
	StartedAt      time.Time      `json:"started_at"`
	CompletedAt    *time.Time     `json:"completed_at,omitempty"`
	ClaudeSession  string         `json:"claude_session,omitempty"`
	ChatSession    string         `json:"chat_session,omitempty"`
	Error          string         `json:"error,omitempty"`
	Workflow       *WorkflowState `json:"workflow,omitempty"`
	NotesFile      string         `json:"notes_file,omitempty"`
	ContextFiles   []string       `json:"context_files,omitempty"`
	TokensIn       int            `json:"tokens_in"`
	TokensOut      int            `json:"tokens_out"`

	// Config fields (persisted for resume)
	Model        string `json:"model,omitempty"`
	CheckerModel string `json:"checker_model,omitempty"`
	WorkflowName string `json:"workflow_name,omitempty"`
	SystemPrompt string `json:"system_prompt,omitempty"`

	// Runtime fields (not persisted)
	mu           sync.RWMutex     `json:"-"`
	cancelFunc   func()           `json:"-"`
	outputBuffer *RingBuffer      `json:"-"`
	subscribers  []chan OutputMsg `json:"-"`
	done         chan struct{}    `json:"-"` // Closed when session completes
}

// Clone returns a thread-safe copy of the session for serialization.
// This method acquires the session's read lock to ensure consistent data.
func (s *Session) Clone() *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clone := &Session{
		ID:            s.ID,
		SpecFiles:     make([]string, len(s.SpecFiles)),
		Status:        s.Status,
		WorkingDir:    s.WorkingDir,
		Iteration:     s.Iteration,
		MaxIterations: s.MaxIterations,
		TotalCost:     s.TotalCost,
		MaxBudget:     s.MaxBudget,
		StartedAt:     s.StartedAt,
		ClaudeSession: s.ClaudeSession,
		ChatSession:   s.ChatSession,
		Error:         s.Error,
		NotesFile:     s.NotesFile,
		TokensIn:      s.TokensIn,
		TokensOut:     s.TokensOut,
		Model:         s.Model,
		CheckerModel:  s.CheckerModel,
		WorkflowName:  s.WorkflowName,
		SystemPrompt:  s.SystemPrompt,
	}

	copy(clone.SpecFiles, s.SpecFiles)

	if s.CompletedAt != nil {
		t := *s.CompletedAt
		clone.CompletedAt = &t
	}

	if s.Worktree != nil {
		clone.Worktree = &WorktreeInfo{
			Name:           s.Worktree.Name,
			Path:           s.Worktree.Path,
			Branch:         s.Worktree.Branch,
			OriginalBranch: s.Worktree.OriginalBranch,
		}
	}

	if s.ContextFiles != nil {
		clone.ContextFiles = make([]string, len(s.ContextFiles))
		copy(clone.ContextFiles, s.ContextFiles)
	}

	if s.Workflow != nil {
		clone.Workflow = &WorkflowState{
			PresetName:       s.Workflow.PresetName,
			CurrentStepIndex: s.Workflow.CurrentStepIndex,
		}
		if s.Workflow.Steps != nil {
			clone.Workflow.Steps = make([]workflow.Step, len(s.Workflow.Steps))
			copy(clone.Workflow.Steps, s.Workflow.Steps)
		}
		if s.Workflow.GateRetries != nil {
			clone.Workflow.GateRetries = make(map[string]int)
			for k, v := range s.Workflow.GateRetries {
				clone.Workflow.GateRetries[k] = v
			}
		}
	}

	return clone
}

// OutputMsg represents a message in the session output stream.
type OutputMsg struct {
	Type      string    `json:"type"` // "text", "tool", "stats", "error"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// RingBuffer is a fixed-size circular buffer for output lines.
type RingBuffer struct {
	mu       sync.RWMutex
	items    []OutputMsg
	size     int
	writePos int
	count    int
}

// NewRingBuffer creates a new ring buffer with the specified capacity.
func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		items: make([]OutputMsg, size),
		size:  size,
	}
}

// Write adds an item to the buffer.
func (rb *RingBuffer) Write(msg OutputMsg) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.items[rb.writePos] = msg
	rb.writePos = (rb.writePos + 1) % rb.size
	if rb.count < rb.size {
		rb.count++
	}
}

// ReadAll returns all items in the buffer in order.
func (rb *RingBuffer) ReadAll() []OutputMsg {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	result := make([]OutputMsg, rb.count)
	if rb.count == 0 {
		return result
	}

	startPos := 0
	if rb.count == rb.size {
		startPos = rb.writePos
	}

	for i := 0; i < rb.count; i++ {
		result[i] = rb.items[(startPos+i)%rb.size]
	}
	return result
}

// DaemonConfig holds daemon configuration.
type DaemonConfig struct {
	NotificationsEnabled bool    `json:"notifications_enabled"`
	NotificationSound    bool    `json:"notification_sound"`
	DefaultBudget        float64 `json:"default_budget"`
	DefaultWorkflow      string  `json:"default_workflow"`
	DefaultWorktree      bool    `json:"default_worktree"`
	ChatBudget           float64 `json:"chat_budget"`
	ChatModel            string  `json:"chat_model"`
}

// DefaultDaemonConfig returns the default daemon configuration.
func DefaultDaemonConfig() *DaemonConfig {
	return &DaemonConfig{
		NotificationsEnabled: true,
		NotificationSound:    false,
		DefaultBudget:        50.0,
		DefaultWorkflow:      "spec-driven",
		DefaultWorktree:      false,
		ChatBudget:           10.0,
		ChatModel:            "sonnet",
	}
}

// StartSessionRequest is the request body for starting a new session.
type StartSessionRequest struct {
	SpecFiles     []string `json:"spec_files"`
	ContextFiles  []string `json:"context_files,omitempty"`
	NotesFile     string   `json:"notes_file,omitempty"`
	Worktree      bool     `json:"worktree"`
	WorktreeName  string   `json:"worktree_name,omitempty"`
	Budget        float64  `json:"budget"`
	MaxIterations int      `json:"max_iterations"`
	Model         string   `json:"model,omitempty"`
	CheckerModel  string   `json:"checker_model,omitempty"`
	Workflow      string   `json:"workflow,omitempty"`
	SystemPrompt  string   `json:"system_prompt,omitempty"`
}

// SessionResponse is the response for session operations.
type SessionResponse struct {
	Session *Session `json:"session,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// SessionListResponse is the response for listing sessions.
type SessionListResponse struct {
	Sessions []*Session `json:"sessions"`
	Total    int        `json:"total"`
}

// ChatRequest is the request body for chat messages.
type ChatRequest struct {
	Message string `json:"message"`
}

// ChatResponse is the response for chat operations.
type ChatResponse struct {
	Response string `json:"response,omitempty"`
	Error    string `json:"error,omitempty"`
}

// DaemonStatus represents the overall daemon status.
type DaemonStatus struct {
	PID        int           `json:"pid"`
	StartedAt  time.Time     `json:"started_at"`
	ProjectDir string        `json:"project_dir"`
	Sessions   SessionCounts `json:"sessions"`
	TotalCost  float64       `json:"total_cost"`
}

// SessionCounts holds counts of sessions by status.
type SessionCounts struct {
	Running   int `json:"running"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
	Stopped   int `json:"stopped"`
	Total     int `json:"total"`
}
