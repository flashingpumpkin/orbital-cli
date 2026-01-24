package main

import (
	"testing"
)

func TestWorktreeFlag_Exists(t *testing.T) {
	// Test that --worktree flag is registered on rootCmd
	flag := rootCmd.PersistentFlags().Lookup("worktree")
	if flag == nil {
		t.Error("--worktree flag is not registered on rootCmd")
	}
}

func TestWorktreeFlag_IsBoolType(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("worktree")
	if flag == nil {
		t.Skip("--worktree flag does not exist yet")
	}
	if flag.Value.Type() != "bool" {
		t.Errorf("--worktree flag type = %q; want %q", flag.Value.Type(), "bool")
	}
}

func TestWorktreeFlag_DefaultsToFalse(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("worktree")
	if flag == nil {
		t.Skip("--worktree flag does not exist yet")
	}
	if flag.DefValue != "false" {
		t.Errorf("--worktree default = %q; want %q", flag.DefValue, "false")
	}
}
