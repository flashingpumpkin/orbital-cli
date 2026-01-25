// Package session provides session management for orbital.
// It provides an interface for session discovery, validation, and selection.
package session

import (
	"time"

	"github.com/flashingpumpkin/orbital/internal/state"
)

// SessionType distinguishes between session sources.
type SessionType int

const (
	// SessionTypeRegular represents a regular session.
	SessionTypeRegular SessionType = iota
)

// Session represents a resumable orbital session.
type Session struct {
	// ID is the Claude session ID for resumption.
	ID string

	// Type indicates the session type.
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

	// RegularState holds the underlying regular state.
	RegularState *state.State
}

// DisplayName returns a human-readable name for the session.
func (s *Session) DisplayName() string {
	if s.Name != "" {
		return s.Name
	}
	return "Main session"
}

// TypeLabel returns the session type label.
func (s *Session) TypeLabel() string {
	return "regular"
}

// Branch returns an empty string (no branch for regular sessions).
func (s *Session) Branch() string {
	return ""
}

// Path returns the working directory path for the session.
func (s *Session) Path() string {
	if s.RegularState != nil {
		return s.RegularState.WorkingDir
	}
	return ""
}
