package session

import (
	"testing"
	"time"

	"github.com/flashingpumpkin/orbital/internal/state"
	"github.com/flashingpumpkin/orbital/internal/worktree"
)

func TestSession_DisplayName(t *testing.T) {
	tests := []struct {
		name     string
		session  Session
		expected string
	}{
		{
			name: "named worktree session",
			session: Session{
				Type: SessionTypeWorktree,
				Name: "swift-falcon",
			},
			expected: "swift-falcon",
		},
		{
			name: "unnamed worktree session",
			session: Session{
				Type: SessionTypeWorktree,
				Name: "",
			},
			expected: "Unnamed worktree",
		},
		{
			name: "named regular session",
			session: Session{
				Type: SessionTypeRegular,
				Name: "Main session",
			},
			expected: "Main session",
		},
		{
			name: "unnamed regular session",
			session: Session{
				Type: SessionTypeRegular,
				Name: "",
			},
			expected: "Main session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.session.DisplayName()
			if got != tt.expected {
				t.Errorf("DisplayName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSession_TypeLabel(t *testing.T) {
	tests := []struct {
		name     string
		session  Session
		expected string
	}{
		{
			name:     "worktree session",
			session:  Session{Type: SessionTypeWorktree},
			expected: "worktree",
		},
		{
			name:     "regular session",
			session:  Session{Type: SessionTypeRegular},
			expected: "regular",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.session.TypeLabel()
			if got != tt.expected {
				t.Errorf("TypeLabel() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSession_Branch(t *testing.T) {
	tests := []struct {
		name     string
		session  Session
		expected string
	}{
		{
			name: "worktree with branch",
			session: Session{
				Type: SessionTypeWorktree,
				WorktreeState: &worktree.WorktreeState{
					Branch: "feature/new-thing",
				},
			},
			expected: "feature/new-thing",
		},
		{
			name: "worktree without state",
			session: Session{
				Type:          SessionTypeWorktree,
				WorktreeState: nil,
			},
			expected: "",
		},
		{
			name: "regular session",
			session: Session{
				Type: SessionTypeRegular,
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.session.Branch()
			if got != tt.expected {
				t.Errorf("Branch() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSession_Path(t *testing.T) {
	tests := []struct {
		name     string
		session  Session
		expected string
	}{
		{
			name: "worktree session",
			session: Session{
				Type: SessionTypeWorktree,
				WorktreeState: &worktree.WorktreeState{
					Path: "/tmp/worktree-path",
				},
			},
			expected: "/tmp/worktree-path",
		},
		{
			name: "regular session",
			session: Session{
				Type: SessionTypeRegular,
				RegularState: &state.State{
					WorkingDir: "/home/user/project",
				},
			},
			expected: "/home/user/project",
		},
		{
			name: "worktree without state",
			session: Session{
				Type:          SessionTypeWorktree,
				WorktreeState: nil,
			},
			expected: "",
		},
		{
			name: "regular without state",
			session: Session{
				Type:         SessionTypeRegular,
				RegularState: nil,
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.session.Path()
			if got != tt.expected {
				t.Errorf("Path() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSessionType_Constants(t *testing.T) {
	// Verify constants have distinct values
	if SessionTypeRegular == SessionTypeWorktree {
		t.Error("SessionTypeRegular and SessionTypeWorktree should have distinct values")
	}

	// Verify SessionTypeRegular is the zero value (first in iota)
	var defaultType SessionType
	if defaultType != SessionTypeRegular {
		t.Error("SessionTypeRegular should be the zero value")
	}
}

func TestSession_FullSession(t *testing.T) {
	// Test a fully populated worktree session
	now := time.Now()
	wtState := &worktree.WorktreeState{
		Name:           "test-worktree",
		Path:           "/tmp/test-path",
		Branch:         "feature/test",
		OriginalBranch: "main",
		SpecFiles:      []string{"spec.md"},
		SessionID:      "session-123",
		CreatedAt:      now,
	}

	s := Session{
		ID:            "session-123",
		Type:          SessionTypeWorktree,
		Name:          "test-worktree",
		SpecFiles:     []string{"spec.md"},
		CreatedAt:     now,
		Valid:         true,
		InvalidReason: "",
		WorktreeState: wtState,
	}

	if s.DisplayName() != "test-worktree" {
		t.Errorf("DisplayName() = %q, want %q", s.DisplayName(), "test-worktree")
	}
	if s.TypeLabel() != "worktree" {
		t.Errorf("TypeLabel() = %q, want %q", s.TypeLabel(), "worktree")
	}
	if s.Branch() != "feature/test" {
		t.Errorf("Branch() = %q, want %q", s.Branch(), "feature/test")
	}
	if s.Path() != "/tmp/test-path" {
		t.Errorf("Path() = %q, want %q", s.Path(), "/tmp/test-path")
	}
}
