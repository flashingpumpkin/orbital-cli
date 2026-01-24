package main

import (
	"testing"
)

func TestWorktreeFlag(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("worktree")
	if flag == nil {
		t.Fatal("--worktree flag is not registered on rootCmd")
	}

	t.Run("is bool type", func(t *testing.T) {
		if flag.Value.Type() != "bool" {
			t.Errorf("--worktree flag type = %q; want %q", flag.Value.Type(), "bool")
		}
	})

	t.Run("defaults to false", func(t *testing.T) {
		if flag.DefValue != "false" {
			t.Errorf("--worktree default = %q; want %q", flag.DefValue, "false")
		}
	})
}
