// Package session provides unified session management for orbital.
// It abstracts over worktree and regular sessions, providing a common
// interface for session discovery, validation, and selection.
package session

import (
	"time"

	"github.com/flashingpumpkin/orbital/internal/state"
	"github.com/flashingpumpkin/orbital/internal/worktree"
)

// SessionType distinguishes between session sources.
type SessionType int

const (
	// SessionTypeRegular represents a non-worktree session.
	SessionTypeRegular SessionType = iota
	// SessionTypeWorktree represents a git worktree session.
	SessionTypeWorktree
)

// Session represents a resumable orbital session (worktree or regular).
type Session struct {
	// ID is the Claude session ID for resumption.
	ID string

	// Type indicates whether this is a worktree or regular session.
	Type SessionType

	// Name is a display name for the session.
	Name string

	// SpecFiles contains the spec files associated with this session.
	SpecFiles []string

	// CreatedAt is when the session was created.
	CreatedAt time.Time

	// Valid indicates whether the session can be resumed.
	Valid bool

	// InvalidReason explains why the session is invalid (if Valid is false).
	InvalidReason string

	// WorktreeState holds the underlying worktree state (if Type is SessionTypeWorktree).
	WorktreeState *worktree.WorktreeState

	// RegularState holds the underlying regular state (if Type is SessionTypeRegular).
	RegularState *state.State
}

// DisplayName returns a human-readable name for the session.
func (s *Session) DisplayName() string {
	if s.Name != "" {
		return s.Name
	}
	if s.Type == SessionTypeWorktree {
		return "Unnamed worktree"
	}
	return "Main session"
}

// TypeLabel returns "worktree" or "regular".
func (s *Session) TypeLabel() string {
	if s.Type == SessionTypeWorktree {
		return "worktree"
	}
	return "regular"
}

// Branch returns the git branch name for worktree sessions, or empty string for regular sessions.
func (s *Session) Branch() string {
	if s.Type == SessionTypeWorktree && s.WorktreeState != nil {
		return s.WorktreeState.Branch
	}
	return ""
}

// Path returns the working directory path for the session.
func (s *Session) Path() string {
	if s.Type == SessionTypeWorktree && s.WorktreeState != nil {
		return s.WorktreeState.Path
	}
	if s.RegularState != nil {
		return s.RegularState.WorkingDir
	}
	return ""
}
