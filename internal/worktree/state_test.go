package worktree

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateManager(t *testing.T) {
	t.Run("Load returns empty state when file does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		state, err := mgr.Load()
		if err != nil {
			t.Fatalf("Load() error = %v; want nil", err)
		}

		if len(state.Worktrees) != 0 {
			t.Errorf("Worktrees = %d; want 0", len(state.Worktrees))
		}
	})

	t.Run("Add persists worktree to state", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		wt := WorktreeState{
			Path:           ".orbit/worktrees/test-feature",
			Branch:         "orbit/test-feature",
			OriginalBranch: "main",
			SpecFiles:      []string{"docs/plans/test.md"},
			SessionID:      "abc123",
		}

		if err := mgr.Add(wt); err != nil {
			t.Fatalf("Add() error = %v; want nil", err)
		}

		// Reload and verify
		state, err := mgr.Load()
		if err != nil {
			t.Fatalf("Load() error = %v; want nil", err)
		}

		if len(state.Worktrees) != 1 {
			t.Fatalf("Worktrees = %d; want 1", len(state.Worktrees))
		}

		got := state.Worktrees[0]
		if got.Path != wt.Path {
			t.Errorf("Path = %q; want %q", got.Path, wt.Path)
		}
		if got.Branch != wt.Branch {
			t.Errorf("Branch = %q; want %q", got.Branch, wt.Branch)
		}
		if got.OriginalBranch != wt.OriginalBranch {
			t.Errorf("OriginalBranch = %q; want %q", got.OriginalBranch, wt.OriginalBranch)
		}
		if got.CreatedAt.IsZero() {
			t.Error("CreatedAt should be set automatically")
		}
	})

	t.Run("Add sets CreatedAt when not provided", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		before := time.Now()
		wt := WorktreeState{
			Path:   ".orbit/worktrees/test",
			Branch: "orbit/test",
		}
		if err := mgr.Add(wt); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		after := time.Now()

		state, _ := mgr.Load()
		createdAt := state.Worktrees[0].CreatedAt

		if createdAt.Before(before) || createdAt.After(after) {
			t.Errorf("CreatedAt = %v; want between %v and %v", createdAt, before, after)
		}
	})

	t.Run("Add preserves provided CreatedAt", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		specificTime := time.Date(2026, 1, 24, 10, 30, 0, 0, time.UTC)
		wt := WorktreeState{
			Path:      ".orbit/worktrees/test",
			Branch:    "orbit/test",
			CreatedAt: specificTime,
		}
		if err := mgr.Add(wt); err != nil {
			t.Fatalf("Add() error = %v", err)
		}

		state, _ := mgr.Load()
		if !state.Worktrees[0].CreatedAt.Equal(specificTime) {
			t.Errorf("CreatedAt = %v; want %v", state.Worktrees[0].CreatedAt, specificTime)
		}
	})

	t.Run("Remove deletes worktree from state", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		// Add two worktrees
		_ = mgr.Add(WorktreeState{Path: ".orbit/worktrees/first", Branch: "orbit/first"})
		_ = mgr.Add(WorktreeState{Path: ".orbit/worktrees/second", Branch: "orbit/second"})

		// Remove the first one
		if err := mgr.Remove(".orbit/worktrees/first"); err != nil {
			t.Fatalf("Remove() error = %v; want nil", err)
		}

		state, _ := mgr.Load()
		if len(state.Worktrees) != 1 {
			t.Fatalf("Worktrees = %d; want 1", len(state.Worktrees))
		}

		if state.Worktrees[0].Path != ".orbit/worktrees/second" {
			t.Errorf("remaining worktree Path = %q; want %q", state.Worktrees[0].Path, ".orbit/worktrees/second")
		}
	})

	t.Run("FindBySpecFile returns matching worktrees", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		_ = mgr.Add(WorktreeState{
			Path:      ".orbit/worktrees/first",
			SpecFiles: []string{"docs/plans/user-auth.md"},
		})
		_ = mgr.Add(WorktreeState{
			Path:      ".orbit/worktrees/second",
			SpecFiles: []string{"docs/plans/other.md"},
		})
		_ = mgr.Add(WorktreeState{
			Path:      ".orbit/worktrees/third",
			SpecFiles: []string{"docs/plans/user-auth.md", "docs/plans/other.md"},
		})

		matches, err := mgr.FindBySpecFile("docs/plans/user-auth.md")
		if err != nil {
			t.Fatalf("FindBySpecFile() error = %v; want nil", err)
		}

		if len(matches) != 2 {
			t.Fatalf("matches = %d; want 2", len(matches))
		}

		paths := map[string]bool{}
		for _, m := range matches {
			paths[m.Path] = true
		}

		if !paths[".orbit/worktrees/first"] || !paths[".orbit/worktrees/third"] {
			t.Errorf("unexpected matches: %v", matches)
		}
	})

	t.Run("List returns all worktrees", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		_ = mgr.Add(WorktreeState{Path: ".orbit/worktrees/first"})
		_ = mgr.Add(WorktreeState{Path: ".orbit/worktrees/second"})

		list, err := mgr.List()
		if err != nil {
			t.Fatalf("List() error = %v; want nil", err)
		}

		if len(list) != 2 {
			t.Errorf("List() = %d items; want 2", len(list))
		}
	})

	t.Run("FindByPath returns matching worktree", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		_ = mgr.Add(WorktreeState{Path: ".orbit/worktrees/target", Branch: "orbit/target"})
		_ = mgr.Add(WorktreeState{Path: ".orbit/worktrees/other", Branch: "orbit/other"})

		found, err := mgr.FindByPath(".orbit/worktrees/target")
		if err != nil {
			t.Fatalf("FindByPath() error = %v; want nil", err)
		}

		if found == nil {
			t.Fatal("FindByPath() = nil; want worktree")
		}

		if found.Branch != "orbit/target" {
			t.Errorf("Branch = %q; want %q", found.Branch, "orbit/target")
		}
	})

	t.Run("FindByPath returns nil when not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		_ = mgr.Add(WorktreeState{Path: ".orbit/worktrees/other"})

		found, err := mgr.FindByPath(".orbit/worktrees/nonexistent")
		if err != nil {
			t.Fatalf("FindByPath() error = %v; want nil", err)
		}

		if found != nil {
			t.Errorf("FindByPath() = %v; want nil", found)
		}
	})

	t.Run("UpdateSessionID updates existing worktree", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		_ = mgr.Add(WorktreeState{Path: ".orbit/worktrees/test", SessionID: "old-session"})

		if err := mgr.UpdateSessionID(".orbit/worktrees/test", "new-session"); err != nil {
			t.Fatalf("UpdateSessionID() error = %v; want nil", err)
		}

		found, _ := mgr.FindByPath(".orbit/worktrees/test")
		if found.SessionID != "new-session" {
			t.Errorf("SessionID = %q; want %q", found.SessionID, "new-session")
		}
	})

	t.Run("UpdateSessionID returns error when not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		err := mgr.UpdateSessionID(".orbit/worktrees/nonexistent", "session")
		if err == nil {
			t.Fatal("UpdateSessionID() error = nil; want error")
		}
	})

	t.Run("StatePath returns correct location", func(t *testing.T) {
		mgr := NewStateManager("/project")

		want := filepath.Join("/project", ".orbit", "worktree-state.json")
		if got := mgr.StatePath(); got != want {
			t.Errorf("StatePath() = %q; want %q", got, want)
		}
	})

	t.Run("creates .orbit directory if needed", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		orbitDir := filepath.Join(tmpDir, ".orbit")
		if _, err := os.Stat(orbitDir); !os.IsNotExist(err) {
			t.Fatal(".orbit directory should not exist before Save")
		}

		_ = mgr.Add(WorktreeState{Path: "test"})

		if _, err := os.Stat(orbitDir); os.IsNotExist(err) {
			t.Error(".orbit directory should exist after Save")
		}
	})
}
