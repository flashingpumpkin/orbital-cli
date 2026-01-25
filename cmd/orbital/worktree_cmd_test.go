package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/flashingpumpkin/orbital/internal/worktree"
)

func TestWorktreeCommands(t *testing.T) {
	t.Run("worktree command exists", func(t *testing.T) {
		if worktreeCmd == nil {
			t.Fatal("worktreeCmd should not be nil")
		}
		if worktreeCmd.Use != "worktree" {
			t.Errorf("worktreeCmd.Use = %q; want %q", worktreeCmd.Use, "worktree")
		}
	})

	t.Run("list subcommand exists", func(t *testing.T) {
		found := false
		for _, cmd := range worktreeCmd.Commands() {
			if cmd.Use == "list" {
				found = true
				break
			}
		}
		if !found {
			t.Error("worktree list subcommand should exist")
		}
	})

	t.Run("show subcommand exists", func(t *testing.T) {
		found := false
		for _, cmd := range worktreeCmd.Commands() {
			if cmd.Use == "show <name>" {
				found = true
				break
			}
		}
		if !found {
			t.Error("worktree show subcommand should exist")
		}
	})

	t.Run("remove subcommand exists", func(t *testing.T) {
		found := false
		for _, cmd := range worktreeCmd.Commands() {
			if cmd.Use == "remove <name>" {
				found = true
				break
			}
		}
		if !found {
			t.Error("worktree remove subcommand should exist")
		}
	})

	t.Run("cleanup subcommand exists", func(t *testing.T) {
		found := false
		for _, cmd := range worktreeCmd.Commands() {
			if cmd.Use == "cleanup" {
				found = true
				break
			}
		}
		if !found {
			t.Error("worktree cleanup subcommand should exist")
		}
	})
}

func TestWorktreeListFlags(t *testing.T) {
	t.Run("--json flag", func(t *testing.T) {
		flag := worktreeListCmd.Flags().Lookup("json")
		if flag == nil {
			t.Fatal("--json flag should exist")
		}
		if flag.Value.Type() != "bool" {
			t.Errorf("--json flag type = %q; want bool", flag.Value.Type())
		}
	})
}

func TestWorktreeRemoveFlags(t *testing.T) {
	t.Run("--force flag", func(t *testing.T) {
		flag := worktreeRemoveCmd.Flags().Lookup("force")
		if flag == nil {
			t.Fatal("--force flag should exist")
		}
		if flag.Value.Type() != "bool" {
			t.Errorf("--force flag type = %q; want bool", flag.Value.Type())
		}
		if flag.Shorthand != "f" {
			t.Errorf("--force shorthand = %q; want %q", flag.Shorthand, "f")
		}
	})
}

func TestWorktreeCleanupFlags(t *testing.T) {
	t.Run("--dry-run flag", func(t *testing.T) {
		flag := worktreeCleanupCmd.Flags().Lookup("dry-run")
		if flag == nil {
			t.Fatal("--dry-run flag should exist")
		}
		if flag.Value.Type() != "bool" {
			t.Errorf("--dry-run flag type = %q; want bool", flag.Value.Type())
		}
	})

	t.Run("--force flag", func(t *testing.T) {
		flag := worktreeCleanupCmd.Flags().Lookup("force")
		if flag == nil {
			t.Fatal("--force flag should exist")
		}
		if flag.Value.Type() != "bool" {
			t.Errorf("--force flag type = %q; want bool", flag.Value.Type())
		}
		if flag.Shorthand != "f" {
			t.Errorf("--force shorthand = %q; want %q", flag.Shorthand, "f")
		}
	})
}

func TestWorktreeListEmpty(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "orbital-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save and restore working directory
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Run list command
	var buf bytes.Buffer
	worktreeListCmd.SetOut(&buf)

	// Reset json flag
	jsonOutput = false

	err = runWorktreeList(worktreeListCmd, []string{})
	if err != nil {
		t.Fatalf("runWorktreeList() error = %v", err)
	}

	output := buf.String()
	if output != "No active worktrees\n" {
		t.Errorf("output = %q; want %q", output, "No active worktrees\n")
	}
}

func TestWorktreeListJSON(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "orbital-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save and restore working directory
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Run list command with JSON output
	var buf bytes.Buffer
	worktreeListCmd.SetOut(&buf)

	jsonOutput = true
	defer func() { jsonOutput = false }()

	err = runWorktreeList(worktreeListCmd, []string{})
	if err != nil {
		t.Fatalf("runWorktreeList() error = %v", err)
	}

	output := buf.String()
	if output != "[]\n" {
		t.Errorf("JSON output = %q; want %q", output, "[]\n")
	}
}

func TestWorktreeShowNotFound(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "orbital-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save and restore working directory
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Run show command for non-existent worktree
	err = runWorktreeShow(worktreeShowCmd, []string{"nonexistent"})
	if err == nil {
		t.Error("runWorktreeShow() should return error for non-existent worktree")
	}
	if err.Error() != "worktree not found: nonexistent" {
		t.Errorf("error = %q; want %q", err.Error(), "worktree not found: nonexistent")
	}
}

func TestWorktreeRemoveNotFound(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "orbital-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Save and restore working directory
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()
	_ = os.Chdir(tmpDir)

	// Run remove command for non-existent worktree
	err = runWorktreeRemove(worktreeRemoveCmd, []string{"nonexistent"})
	if err == nil {
		t.Error("runWorktreeRemove() should return error for non-existent worktree")
	}
	if err.Error() != "worktree not found: nonexistent" {
		t.Errorf("error = %q; want %q", err.Error(), "worktree not found: nonexistent")
	}
}

func TestGetWorktreeStatus(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "orbital-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("missing path", func(t *testing.T) {
		wt := &worktree.WorktreeState{
			Name: "test",
			Path: filepath.Join(tmpDir, "nonexistent"),
		}
		status := getWorktreeStatus(tmpDir, wt)
		if status != "MISSING" {
			t.Errorf("status = %q; want %q", status, "MISSING")
		}
	})

	t.Run("existing path not a worktree", func(t *testing.T) {
		subDir := filepath.Join(tmpDir, "subdir")
		_ = os.MkdirAll(subDir, 0755)

		wt := &worktree.WorktreeState{
			Name: "test",
			Path: subDir,
		}
		status := getWorktreeStatus(tmpDir, wt)
		// Not a git worktree, so should be ORPHAN
		if status != "ORPHAN" {
			t.Errorf("status = %q; want %q", status, "ORPHAN")
		}
	})
}
