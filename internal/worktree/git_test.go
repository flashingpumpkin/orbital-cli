package worktree

import (
	"strings"
	"testing"
)

func TestValidateBranchName(t *testing.T) {
	tests := []struct {
		name        string
		branchName  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "valid branch name",
			branchName: "orbital/swift-falcon",
			wantErr:    false,
		},
		{
			name:       "valid branch name with numbers",
			branchName: "orbital/swift-falcon-2",
			wantErr:    false,
		},
		{
			name:       "valid branch name with multiple hyphens",
			branchName: "orbital/fix-user-auth-bug",
			wantErr:    false,
		},
		{
			name:        "missing orbital prefix",
			branchName:  "swift-falcon",
			wantErr:     true,
			errContains: "must start with",
		},
		{
			name:        "wrong prefix",
			branchName:  "orbit/swift-falcon",
			wantErr:     true,
			errContains: "must start with",
		},
		{
			name:        "contains spaces",
			branchName:  "orbital/swift falcon",
			wantErr:     true,
			errContains: "contains spaces",
		},
		{
			name:        "contains invalid characters",
			branchName:  "orbital/swift_falcon",
			wantErr:     true,
			errContains: "invalid characters",
		},
		{
			name:        "corrupted with success suffix",
			branchName:  "orbital/fix-formattingsuccess",
			wantErr:     true,
			errContains: "corrupted",
		},
		{
			name:        "corrupted with true suffix",
			branchName:  "orbital/swift-falcontrue",
			wantErr:     true,
			errContains: "corrupted",
		},
		{
			name:       "valid with success as hyphenated word",
			branchName: "orbital/fix-success",
			wantErr:    false,
		},
		{
			name:       "valid ending with true as hyphenated suffix",
			branchName: "orbital/set-flag-true",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBranchName(tt.branchName)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateBranchName(%q) = nil; want error", tt.branchName)
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateBranchName(%q) error = %v; want error containing %q", tt.branchName, err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateBranchName(%q) = %v; want nil", tt.branchName, err)
				}
			}
		})
	}
}

func TestValidateWorktreeName(t *testing.T) {
	tests := []struct {
		name        string
		wtName      string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid adjective-animal name",
			wtName:  "swift-falcon",
			wantErr: false,
		},
		{
			name:    "valid name with numbers",
			wtName:  "swift-falcon-2",
			wantErr: false,
		},
		{
			name:    "valid single word",
			wtName:  "feature",
			wantErr: false,
		},
		{
			name:        "empty name",
			wtName:      "",
			wantErr:     true,
			errContains: "cannot be empty",
		},
		{
			name:        "uppercase letters",
			wtName:      "Swift-Falcon",
			wantErr:     true,
			errContains: "lowercase",
		},
		{
			name:        "starts with hyphen",
			wtName:      "-falcon",
			wantErr:     true,
			errContains: "lowercase",
		},
		{
			name:        "ends with hyphen",
			wtName:      "falcon-",
			wantErr:     true,
			errContains: "lowercase",
		},
		{
			name:        "contains underscore",
			wtName:      "swift_falcon",
			wantErr:     true,
			errContains: "lowercase",
		},
		{
			name:        "contains spaces",
			wtName:      "swift falcon",
			wantErr:     true,
			errContains: "lowercase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorktreeName(tt.wtName)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateWorktreeName(%q) = nil; want error", tt.wtName)
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateWorktreeName(%q) error = %v; want error containing %q", tt.wtName, err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateWorktreeName(%q) = %v; want nil", tt.wtName, err)
				}
			}
		})
	}
}

func TestWorktreePath(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"swift-falcon", ".orbital/worktrees/swift-falcon"},
		{"calm-otter", ".orbital/worktrees/calm-otter"},
		{"my-feature-2", ".orbital/worktrees/my-feature-2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WorktreePath(tt.name)
			if got != tt.want {
				t.Errorf("WorktreePath(%q) = %q; want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestBranchName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"swift-falcon", "orbital/swift-falcon"},
		{"calm-otter", "orbital/calm-otter"},
		{"my-feature-2", "orbital/my-feature-2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BranchName(tt.name)
			if got != tt.want {
				t.Errorf("BranchName(%q) = %q; want %q", tt.name, got, tt.want)
			}
		})
	}
}

// Note: Integration tests for CreateWorktree, RemoveWorktree, and DeleteBranch
// require a real git repository and are deferred to integration testing.
