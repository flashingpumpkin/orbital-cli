package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewCollector(t *testing.T) {
	c := NewCollector("/tmp/test")
	if c == nil {
		t.Fatal("NewCollector returned nil")
	}
	if c.workingDir != "/tmp/test" {
		t.Errorf("workingDir = %q, want %q", c.workingDir, "/tmp/test")
	}
}

func TestCollector_Collect_NoSessions(t *testing.T) {
	// Create a temp directory with no state
	tmpDir := t.TempDir()

	c := NewCollector(tmpDir)
	sessions, err := c.Collect()
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("Collect() returned %d sessions, want 0", len(sessions))
	}
}

func TestCollector_Collect_RegularSession(t *testing.T) {
	tmpDir := t.TempDir()

	// Create regular state
	stateDir := filepath.Join(tmpDir, ".orbital", "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Use a very large PID that won't exist on any system.
	// PID 0 doesn't work because on Unix it refers to the calling process's group.
	stateData := map[string]interface{}{
		"session_id":   "regular-session-123",
		"pid":          99999999, // Large PID that won't exist, so IsStale() returns true
		"working_dir":  tmpDir,
		"active_files": []string{"story.md"},
		"started_at":   time.Now().Format(time.RFC3339),
		"iteration":    5,
		"total_cost":   1.23,
	}
	data, _ := json.Marshal(stateData)
	if err := os.WriteFile(filepath.Join(stateDir, "state.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	c := NewCollector(tmpDir)
	sessions, err := c.Collect()
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("Collect() returned %d sessions, want 1", len(sessions))
	}

	s := sessions[0]
	if s.Type != SessionTypeRegular {
		t.Errorf("session Type = %v, want SessionTypeRegular", s.Type)
	}
	if s.ID != "regular-session-123" {
		t.Errorf("session ID = %q, want %q", s.ID, "regular-session-123")
	}
	if !s.Valid {
		t.Errorf("session Valid = false, want true (session should be stale with PID 0)")
	}
}

func TestCollector_ValidSessions(t *testing.T) {
	sessions := []Session{
		{ID: "1", Valid: true},
		{ID: "2", Valid: false, InvalidReason: "deleted"},
		{ID: "3", Valid: true},
		{ID: "4", Valid: false, InvalidReason: "running"},
	}

	c := NewCollector("/tmp")
	valid := c.ValidSessions(sessions)

	if len(valid) != 2 {
		t.Fatalf("ValidSessions() returned %d sessions, want 2", len(valid))
	}

	for _, s := range valid {
		if !s.Valid {
			t.Errorf("ValidSessions() returned invalid session %q", s.ID)
		}
	}
}

func TestCollector_ValidSessions_Empty(t *testing.T) {
	c := NewCollector("/tmp")
	valid := c.ValidSessions(nil)

	if valid != nil {
		t.Errorf("ValidSessions(nil) = %v, want nil", valid)
	}
}

func TestCollector_ValidSessions_AllInvalid(t *testing.T) {
	sessions := []Session{
		{ID: "1", Valid: false},
		{ID: "2", Valid: false},
	}

	c := NewCollector("/tmp")
	valid := c.ValidSessions(sessions)

	if valid != nil {
		t.Errorf("ValidSessions() = %v, want nil when all invalid", valid)
	}
}

func TestCollector_Collect_QueuedFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create queue without regular session
	stateDir := filepath.Join(tmpDir, ".orbital", "state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		t.Fatal(err)
	}

	queueData := map[string]interface{}{
		"queued_files": []string{"queued-spec.md"},
		"added_at":     map[string]string{},
	}
	data, _ := json.Marshal(queueData)
	if err := os.WriteFile(filepath.Join(stateDir, "queue.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	c := NewCollector(tmpDir)
	sessions, err := c.Collect()
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("Collect() returned %d sessions, want 1", len(sessions))
	}

	s := sessions[0]
	if s.Type != SessionTypeRegular {
		t.Errorf("session Type = %v, want SessionTypeRegular", s.Type)
	}
	if s.Name != "Queued files" {
		t.Errorf("session Name = %q, want %q", s.Name, "Queued files")
	}
	if !s.Valid {
		t.Error("queued session should be valid")
	}
	if len(s.SpecFiles) != 1 || s.SpecFiles[0] != "queued-spec.md" {
		t.Errorf("session SpecFiles = %v, want [queued-spec.md]", s.SpecFiles)
	}
}

func TestCollector_hasRegularSession(t *testing.T) {
	c := NewCollector("/tmp")

	tests := []struct {
		name     string
		sessions []Session
		expected bool
	}{
		{
			name:     "empty",
			sessions: nil,
			expected: false,
		},
		{
			name: "has regular",
			sessions: []Session{
				{Type: SessionTypeRegular},
			},
			expected: true,
		},
		{
			name: "multiple regular",
			sessions: []Session{
				{Type: SessionTypeRegular},
				{Type: SessionTypeRegular},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.hasRegularSession(tt.sessions)
			if got != tt.expected {
				t.Errorf("hasRegularSession() = %v, want %v", got, tt.expected)
			}
		})
	}
}
