package main

import (
	"testing"
)

func TestWorktreeFlags(t *testing.T) {
	t.Run("--worktree flag", func(t *testing.T) {
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
	})

	t.Run("--worktree-name flag", func(t *testing.T) {
		flag := rootCmd.PersistentFlags().Lookup("worktree-name")
		if flag == nil {
			t.Fatal("--worktree-name flag is not registered on rootCmd")
		}

		t.Run("is string type", func(t *testing.T) {
			if flag.Value.Type() != "string" {
				t.Errorf("--worktree-name flag type = %q; want %q", flag.Value.Type(), "string")
			}
		})

		t.Run("defaults to empty", func(t *testing.T) {
			if flag.DefValue != "" {
				t.Errorf("--worktree-name default = %q; want empty", flag.DefValue)
			}
		})
	})

	t.Run("--setup-model flag", func(t *testing.T) {
		flag := rootCmd.PersistentFlags().Lookup("setup-model")
		if flag == nil {
			t.Fatal("--setup-model flag is not registered on rootCmd")
		}

		t.Run("is string type", func(t *testing.T) {
			if flag.Value.Type() != "string" {
				t.Errorf("--setup-model flag type = %q; want %q", flag.Value.Type(), "string")
			}
		})

		t.Run("defaults to haiku", func(t *testing.T) {
			if flag.DefValue != "haiku" {
				t.Errorf("--setup-model default = %q; want %q", flag.DefValue, "haiku")
			}
		})
	})

	t.Run("--merge-model flag", func(t *testing.T) {
		flag := rootCmd.PersistentFlags().Lookup("merge-model")
		if flag == nil {
			t.Fatal("--merge-model flag is not registered on rootCmd")
		}

		t.Run("is string type", func(t *testing.T) {
			if flag.Value.Type() != "string" {
				t.Errorf("--merge-model flag type = %q; want %q", flag.Value.Type(), "string")
			}
		})

		t.Run("defaults to haiku", func(t *testing.T) {
			if flag.DefValue != "haiku" {
				t.Errorf("--merge-model default = %q; want %q", flag.DefValue, "haiku")
			}
		})
	})
}
