package session

import (
	"testing"
	"time"

	"github.com/flashingpumpkin/orbital/internal/state"
)

func TestSession_DisplayName(t *testing.T) {
	tests := []struct {
		name     string
		session  Session
		expected string
	}{
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
	// Verify SessionTypeRegular is the zero value (first in iota)
	var defaultType SessionType
	if defaultType != SessionTypeRegular {
		t.Error("SessionTypeRegular should be the zero value")
	}
}

func TestSession_FullSession(t *testing.T) {
	// Test a fully populated regular session
	now := time.Now()
	regState := &state.State{
		SessionID:  "session-123",
		WorkingDir: "/home/user/project",
	}

	s := Session{
		ID:           "session-123",
		Type:         SessionTypeRegular,
		Name:         "Main session",
		SpecFiles:    []string{"spec.md"},
		CreatedAt:    now,
		Valid:        true,
		RegularState: regState,
	}

	if s.DisplayName() != "Main session" {
		t.Errorf("DisplayName() = %q, want %q", s.DisplayName(), "Main session")
	}
	if s.TypeLabel() != "regular" {
		t.Errorf("TypeLabel() = %q, want %q", s.TypeLabel(), "regular")
	}
	if s.Branch() != "" {
		t.Errorf("Branch() = %q, want %q", s.Branch(), "")
	}
	if s.Path() != "/home/user/project" {
		t.Errorf("Path() = %q, want %q", s.Path(), "/home/user/project")
	}
}
