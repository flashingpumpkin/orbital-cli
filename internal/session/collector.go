package session

import (
	"time"

	"github.com/flashingpumpkin/orbital/internal/state"
)

// Collector gathers all available sessions from a working directory.
type Collector struct {
	workingDir string
}

// NewCollector creates a new Collector for the given working directory.
func NewCollector(workingDir string) *Collector {
	return &Collector{workingDir: workingDir}
}

// Collect returns all sessions (valid and invalid) from the working directory.
func (c *Collector) Collect() ([]Session, error) {
	var sessions []Session

	// Collect regular session
	regSession, err := c.collectRegularSession()
	if err == nil && regSession != nil {
		sessions = append(sessions, *regSession)
	}

	// Check for queued files without a regular session
	if !c.hasRegularSession(sessions) {
		queueSession := c.collectQueuedSession()
		if queueSession != nil {
			sessions = append(sessions, *queueSession)
		}
	}

	return sessions, nil
}

// collectRegularSession gathers the regular (non-worktree) session if one exists.
func (c *Collector) collectRegularSession() (*Session, error) {
	if !state.Exists(c.workingDir) {
		return nil, nil
	}

	st, err := state.Load(c.workingDir)
	if err != nil {
		return nil, err
	}

	s := Session{
		ID:           st.SessionID,
		Type:         SessionTypeRegular,
		Name:         "Main session",
		SpecFiles:    st.ActiveFiles,
		CreatedAt:    st.StartedAt,
		RegularState: st,
	}

	// Validate: a session is invalid if it's currently running (not stale)
	if !st.IsStale() {
		s.Valid = false
		s.InvalidReason = "Session is currently running"
	} else {
		s.Valid = true
	}

	return &s, nil
}

// collectQueuedSession creates a synthetic session for queued files if no regular session exists.
func (c *Collector) collectQueuedSession() *Session {
	stateDir := state.StateDir(c.workingDir)
	queue, err := state.LoadQueue(stateDir)
	if err != nil || queue.IsEmpty() {
		return nil
	}

	return &Session{
		ID:        "",
		Type:      SessionTypeRegular,
		Name:      "Queued files",
		SpecFiles: queue.QueuedFiles,
		CreatedAt: time.Now(),
		Valid:     true,
	}
}

// hasRegularSession checks if the sessions slice contains a regular session.
func (c *Collector) hasRegularSession(sessions []Session) bool {
	for _, s := range sessions {
		if s.Type == SessionTypeRegular {
			return true
		}
	}
	return false
}

// ValidSessions filters and returns only valid, resumable sessions.
func (c *Collector) ValidSessions(sessions []Session) []Session {
	var valid []Session
	for _, s := range sessions {
		if s.Valid {
			valid = append(valid, s)
		}
	}
	return valid
}
