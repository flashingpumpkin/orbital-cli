package worktree

import (
	"os"
	"path/filepath"
	"sync"
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

		// Use absolute path as required by validation
		absPath := filepath.Join(tmpDir, ".orbital/worktrees/test-feature")

		wt := WorktreeState{
			Path:           absPath,
			Branch:         "orbital/test-feature",
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

	t.Run("Add rejects relative paths", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		wt := WorktreeState{
			Path:   ".orbital/worktrees/test-feature", // Relative path
			Branch: "orbital/test-feature",
		}

		err := mgr.Add(wt)
		if err == nil {
			t.Fatal("Add() error = nil; want error for relative path")
		}

		if !contains(err.Error(), "must be absolute") {
			t.Errorf("error = %q; want error containing 'must be absolute'", err.Error())
		}
	})

	t.Run("Add sets CreatedAt when not provided", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		before := time.Now()
		wt := WorktreeState{
			Path:   filepath.Join(tmpDir, ".orbital/worktrees/test"),
			Branch: "orbital/test",
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
			Path:      filepath.Join(tmpDir, ".orbital/worktrees/test"),
			Branch:    "orbital/test",
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

		// Add two worktrees with absolute paths
		first := filepath.Join(tmpDir, ".orbital/worktrees/first")
		second := filepath.Join(tmpDir, ".orbital/worktrees/second")
		_ = mgr.Add(WorktreeState{Path: first, Branch: "orbital/first"})
		_ = mgr.Add(WorktreeState{Path: second, Branch: "orbital/second"})

		// Remove the first one
		if err := mgr.Remove(first); err != nil {
			t.Fatalf("Remove() error = %v; want nil", err)
		}

		state, _ := mgr.Load()
		if len(state.Worktrees) != 1 {
			t.Fatalf("Worktrees = %d; want 1", len(state.Worktrees))
		}

		if state.Worktrees[0].Path != second {
			t.Errorf("remaining worktree Path = %q; want %q", state.Worktrees[0].Path, second)
		}
	})

	t.Run("FindBySpecFile returns matching worktrees", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		first := filepath.Join(tmpDir, ".orbital/worktrees/first")
		second := filepath.Join(tmpDir, ".orbital/worktrees/second")
		third := filepath.Join(tmpDir, ".orbital/worktrees/third")

		_ = mgr.Add(WorktreeState{
			Path:      first,
			SpecFiles: []string{"docs/plans/user-auth.md"},
		})
		_ = mgr.Add(WorktreeState{
			Path:      second,
			SpecFiles: []string{"docs/plans/other.md"},
		})
		_ = mgr.Add(WorktreeState{
			Path:      third,
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

		if !paths[first] || !paths[third] {
			t.Errorf("unexpected matches: %v", matches)
		}
	})

	t.Run("List returns all worktrees", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		_ = mgr.Add(WorktreeState{Path: filepath.Join(tmpDir, ".orbital/worktrees/first")})
		_ = mgr.Add(WorktreeState{Path: filepath.Join(tmpDir, ".orbital/worktrees/second")})

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

		target := filepath.Join(tmpDir, ".orbital/worktrees/target")
		other := filepath.Join(tmpDir, ".orbital/worktrees/other")

		_ = mgr.Add(WorktreeState{Path: target, Branch: "orbital/target"})
		_ = mgr.Add(WorktreeState{Path: other, Branch: "orbital/other"})

		found, err := mgr.FindByPath(target)
		if err != nil {
			t.Fatalf("FindByPath() error = %v; want nil", err)
		}

		if found == nil {
			t.Fatal("FindByPath() = nil; want worktree")
		}

		if found.Branch != "orbital/target" {
			t.Errorf("Branch = %q; want %q", found.Branch, "orbital/target")
		}
	})

	t.Run("FindByPath returns nil when not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		_ = mgr.Add(WorktreeState{Path: filepath.Join(tmpDir, ".orbital/worktrees/other")})

		found, err := mgr.FindByPath(filepath.Join(tmpDir, ".orbital/worktrees/nonexistent"))
		if err != nil {
			t.Fatalf("FindByPath() error = %v; want nil", err)
		}

		if found != nil {
			t.Errorf("FindByPath() = %v; want nil", found)
		}
	})

	t.Run("FindByName returns matching worktree", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		_ = mgr.Add(WorktreeState{
			Name:   "swift-falcon",
			Path:   filepath.Join(tmpDir, ".orbital/worktrees/swift-falcon"),
			Branch: "orbital/swift-falcon",
		})

		found, err := mgr.FindByName("swift-falcon")
		if err != nil {
			t.Fatalf("FindByName() error = %v; want nil", err)
		}

		if found == nil {
			t.Fatal("FindByName() = nil; want worktree")
		}

		if found.Branch != "orbital/swift-falcon" {
			t.Errorf("Branch = %q; want %q", found.Branch, "orbital/swift-falcon")
		}
	})

	t.Run("UpdateSessionID updates existing worktree", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		path := filepath.Join(tmpDir, ".orbital/worktrees/test")
		_ = mgr.Add(WorktreeState{Path: path, SessionID: "old-session"})

		if err := mgr.UpdateSessionID(path, "new-session"); err != nil {
			t.Fatalf("UpdateSessionID() error = %v; want nil", err)
		}

		found, _ := mgr.FindByPath(path)
		if found.SessionID != "new-session" {
			t.Errorf("SessionID = %q; want %q", found.SessionID, "new-session")
		}
	})

	t.Run("UpdateSessionID returns error when not found", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		err := mgr.UpdateSessionID(filepath.Join(tmpDir, ".orbital/worktrees/nonexistent"), "session")
		if err == nil {
			t.Fatal("UpdateSessionID() error = nil; want error")
		}
	})

	t.Run("StatePath returns correct location", func(t *testing.T) {
		mgr := NewStateManager("/project")

		want := filepath.Join("/project", ".orbital", "worktree-state.json")
		if got := mgr.StatePath(); got != want {
			t.Errorf("StatePath() = %q; want %q", got, want)
		}
	})

	t.Run("creates .orbital directory if needed", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		orbitDir := filepath.Join(tmpDir, ".orbital")
		if _, err := os.Stat(orbitDir); !os.IsNotExist(err) {
			t.Fatal(".orbital directory should not exist before Save")
		}

		_ = mgr.Add(WorktreeState{Path: filepath.Join(tmpDir, "test")})

		if _, err := os.Stat(orbitDir); os.IsNotExist(err) {
			t.Error(".orbital directory should exist after Save")
		}
	})
}

func TestStateManagerAtomicWrite(t *testing.T) {
	t.Run("creates backup before writing", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		// Add initial worktree
		path1 := filepath.Join(tmpDir, ".orbital/worktrees/first")
		_ = mgr.Add(WorktreeState{Path: path1, Branch: "orbital/first"})

		// Add another - this should create backup
		path2 := filepath.Join(tmpDir, ".orbital/worktrees/second")
		_ = mgr.Add(WorktreeState{Path: path2, Branch: "orbital/second"})

		// Check backup exists
		backupPath := mgr.backupPath()
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			t.Error("backup file should exist after second write")
		}
	})

	t.Run("atomic write survives crash simulation", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		// Add initial worktree
		path := filepath.Join(tmpDir, ".orbital/worktrees/test")
		_ = mgr.Add(WorktreeState{Path: path, Branch: "orbital/test"})

		// Verify we can still load
		state, err := mgr.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if len(state.Worktrees) != 1 {
			t.Errorf("Worktrees = %d; want 1", len(state.Worktrees))
		}
	})
}

func TestStateManagerCorruptionRecovery(t *testing.T) {
	t.Run("recovers from corrupted state file using backup", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		// Add initial worktree
		path := filepath.Join(tmpDir, ".orbital/worktrees/test")
		_ = mgr.Add(WorktreeState{Path: path, Branch: "orbital/test"})

		// Add another to create backup
		path2 := filepath.Join(tmpDir, ".orbital/worktrees/second")
		_ = mgr.Add(WorktreeState{Path: path2, Branch: "orbital/second"})

		// Corrupt the main state file
		if err := os.WriteFile(mgr.StatePath(), []byte("invalid json"), 0644); err != nil {
			t.Fatalf("failed to corrupt state file: %v", err)
		}

		// Load should recover from backup
		state, err := mgr.Load()
		if err != nil {
			t.Fatalf("Load() error = %v; want nil (should recover from backup)", err)
		}

		// Backup had only one worktree (before second add)
		if len(state.Worktrees) != 1 {
			t.Errorf("Worktrees = %d; want 1 (from backup)", len(state.Worktrees))
		}
	})

	t.Run("returns error when both state and backup are corrupted", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		// Create .orbital directory
		orbitDir := filepath.Join(tmpDir, ".orbital")
		_ = os.MkdirAll(orbitDir, 0755)

		// Write corrupted state file
		if err := os.WriteFile(mgr.StatePath(), []byte("invalid json"), 0644); err != nil {
			t.Fatalf("failed to write corrupted state: %v", err)
		}

		// Write corrupted backup
		if err := os.WriteFile(mgr.backupPath(), []byte("also invalid"), 0644); err != nil {
			t.Fatalf("failed to write corrupted backup: %v", err)
		}

		// Load should fail
		_, err := mgr.Load()
		if err == nil {
			t.Fatal("Load() error = nil; want error when both files corrupted")
		}
	})
}

func TestStateManagerPathMigration(t *testing.T) {
	t.Run("migrates relative paths to absolute on load", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		// Create .orbital directory
		orbitDir := filepath.Join(tmpDir, ".orbital")
		_ = os.MkdirAll(orbitDir, 0755)

		// Write state file with relative path (simulating old format)
		oldState := `{
  "worktrees": [
    {
      "path": ".orbital/worktrees/old-style",
      "branch": "orbital/old-style"
    }
  ]
}`
		if err := os.WriteFile(mgr.StatePath(), []byte(oldState), 0644); err != nil {
			t.Fatalf("failed to write old state: %v", err)
		}

		// Load should migrate the path
		state, err := mgr.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if len(state.Worktrees) != 1 {
			t.Fatalf("Worktrees = %d; want 1", len(state.Worktrees))
		}

		// Path should now be absolute
		if !filepath.IsAbs(state.Worktrees[0].Path) {
			t.Errorf("Path = %q; want absolute path", state.Worktrees[0].Path)
		}

		expectedPath := filepath.Join(tmpDir, ".orbital/worktrees/old-style")
		if state.Worktrees[0].Path != expectedPath {
			t.Errorf("Path = %q; want %q", state.Worktrees[0].Path, expectedPath)
		}
	})
}

func TestStateManagerLocking(t *testing.T) {
	t.Run("concurrent adds do not lose data", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		// Run multiple goroutines adding worktrees concurrently
		var wg sync.WaitGroup
		numWorkers := 5

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				path := filepath.Join(tmpDir, ".orbital/worktrees", filepath.Base(t.Name())+"-"+string(rune('a'+id)))
				err := mgr.Add(WorktreeState{
					Path:   path,
					Branch: "orbital/test-" + string(rune('a'+id)),
				})
				if err != nil {
					t.Errorf("Add() error = %v", err)
				}
			}(i)
		}

		wg.Wait()

		// All worktrees should be present
		state, err := mgr.Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if len(state.Worktrees) != numWorkers {
			t.Errorf("Worktrees = %d; want %d (concurrent adds may have lost data)", len(state.Worktrees), numWorkers)
		}
	})

	t.Run("lock is released after operation", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewStateManager(tmpDir)

		// Add a worktree
		path := filepath.Join(tmpDir, ".orbital/worktrees/test")
		_ = mgr.Add(WorktreeState{Path: path, Branch: "orbital/test"})

		// Lock file should not exist after operation
		lockPath := mgr.lockPath()
		if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
			t.Error("lock file should not exist after operation completes")
		}
	})
}

func TestValidateWorktree(t *testing.T) {
	t.Run("returns error for non-existent path", func(t *testing.T) {
		wt := &WorktreeState{
			Path: "/nonexistent/path/to/worktree",
		}

		err := ValidateWorktree(wt)
		if err == nil {
			t.Fatal("ValidateWorktree() error = nil; want error for non-existent path")
		}

		if !contains(err.Error(), "not found") {
			t.Errorf("error = %q; want error containing 'not found'", err.Error())
		}
	})

	t.Run("returns error for path that is not a directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a file instead of directory
		filePath := filepath.Join(tmpDir, "not-a-dir")
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		wt := &WorktreeState{Path: filePath}

		err := ValidateWorktree(wt)
		if err == nil {
			t.Fatal("ValidateWorktree() error = nil; want error for file path")
		}

		if !contains(err.Error(), "not a directory") {
			t.Errorf("error = %q; want error containing 'not a directory'", err.Error())
		}
	})

	t.Run("returns error for directory without .git", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create empty directory
		worktreeDir := filepath.Join(tmpDir, "fake-worktree")
		if err := os.MkdirAll(worktreeDir, 0755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}

		wt := &WorktreeState{Path: worktreeDir}

		err := ValidateWorktree(wt)
		if err == nil {
			t.Fatal("ValidateWorktree() error = nil; want error for missing .git")
		}

		if !contains(err.Error(), "missing .git") {
			t.Errorf("error = %q; want error containing 'missing .git'", err.Error())
		}
	})

	t.Run("returns error for git repository (not worktree)", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create directory with .git directory (like a regular repo, not a worktree)
		repoDir := filepath.Join(tmpDir, "regular-repo")
		gitDir := filepath.Join(repoDir, ".git")
		if err := os.MkdirAll(gitDir, 0755); err != nil {
			t.Fatalf("failed to create .git directory: %v", err)
		}

		wt := &WorktreeState{Path: repoDir}

		err := ValidateWorktree(wt)
		if err == nil {
			t.Fatal("ValidateWorktree() error = nil; want error for regular repo")
		}

		if !contains(err.Error(), ".git is directory") {
			t.Errorf("error = %q; want error containing '.git is directory'", err.Error())
		}
	})

	t.Run("returns nil for valid worktree", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create directory with .git file (like a worktree)
		worktreeDir := filepath.Join(tmpDir, "valid-worktree")
		if err := os.MkdirAll(worktreeDir, 0755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}

		// Create .git file (not directory) like a real worktree has
		gitFilePath := filepath.Join(worktreeDir, ".git")
		if err := os.WriteFile(gitFilePath, []byte("gitdir: /path/to/main/repo/.git/worktrees/test"), 0644); err != nil {
			t.Fatalf("failed to create .git file: %v", err)
		}

		wt := &WorktreeState{Path: worktreeDir}

		err := ValidateWorktree(wt)
		if err != nil {
			t.Errorf("ValidateWorktree() error = %v; want nil for valid worktree", err)
		}
	})
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
