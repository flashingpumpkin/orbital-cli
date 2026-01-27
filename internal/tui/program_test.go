package tui

import (
	"os"
	"testing"
)

func TestNewProgram(t *testing.T) {
	// Skip if running in CI without a terminal
	if os.Getenv("CI") != "" {
		t.Skip("Skipping TUI test in CI environment")
	}

	session := SessionInfo{
		SpecFiles: []string{"test.md"},
		NotesFile: "notes.md",
	}
	progress := ProgressInfo{
		Iteration:    1,
		MaxIteration: 50,
		Budget:       100.0,
	}

	prog := New(session, progress, "auto")

	if prog == nil {
		t.Fatal("expected non-nil Program")
	}

	if prog.Bridge() == nil {
		t.Error("expected non-nil Bridge")
	}

	if prog.bridge.tracker == nil {
		t.Error("expected non-nil TaskTracker in Bridge")
	}
}

func TestProgramBridge(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping TUI test in CI environment")
	}

	session := SessionInfo{}
	progress := ProgressInfo{}

	prog := New(session, progress, "auto")
	bridge := prog.Bridge()

	if bridge == nil {
		t.Fatal("expected non-nil Bridge")
	}

	// Bridge should implement io.Writer
	var _ interface{ Write([]byte) (int, error) } = bridge
}
