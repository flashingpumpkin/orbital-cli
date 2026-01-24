package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCmd_CreatesConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	cmd := newInitCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Check file was created
	configPath := filepath.Join(tempDir, ".orbit", "config.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("config file was not created at %s", configPath)
	}

	// Check output message
	output := buf.String()
	if !strings.Contains(output, "Created") {
		t.Errorf("output = %q; want to contain 'Created'", output)
	}

	// Check file contents
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	if !strings.Contains(string(content), "{{files}}") {
		t.Errorf("config file missing {{files}} placeholder")
	}
	if !strings.Contains(string(content), "[agents.my-agent]") {
		t.Errorf("config file missing agents example")
	}
}

func TestInitCmd_FailsIfConfigExists(t *testing.T) {
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Create existing config
	orbitDir := filepath.Join(tempDir, ".orbit")
	if err := os.MkdirAll(orbitDir, 0755); err != nil {
		t.Fatalf("failed to create .orbit directory: %v", err)
	}
	configPath := filepath.Join(orbitDir, "config.toml")
	if err := os.WriteFile(configPath, []byte("existing config"), 0644); err != nil {
		t.Fatalf("failed to create existing config: %v", err)
	}

	cmd := newInitCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected error when config exists, got nil")
	}

	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q; want to contain 'already exists'", err.Error())
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error = %q; want to contain '--force'", err.Error())
	}

	// Verify original file was not modified
	content, _ := os.ReadFile(configPath)
	if string(content) != "existing config" {
		t.Errorf("existing config was modified")
	}
}

func TestInitCmd_ForceOverwritesExisting(t *testing.T) {
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Create existing config
	orbitDir := filepath.Join(tempDir, ".orbit")
	if err := os.MkdirAll(orbitDir, 0755); err != nil {
		t.Fatalf("failed to create .orbit directory: %v", err)
	}
	configPath := filepath.Join(orbitDir, "config.toml")
	if err := os.WriteFile(configPath, []byte("existing config"), 0644); err != nil {
		t.Fatalf("failed to create existing config: %v", err)
	}

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--force"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() with --force error = %v", err)
	}

	// Verify file was overwritten with template
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	if string(content) == "existing config" {
		t.Errorf("config was not overwritten")
	}
	if !strings.Contains(string(content), "{{files}}") {
		t.Errorf("config file missing {{files}} placeholder after overwrite")
	}
}

func TestInitCmd_CreatesOrbitDirectory(t *testing.T) {
	tempDir := t.TempDir()
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Ensure .orbit doesn't exist
	orbitDir := filepath.Join(tempDir, ".orbit")
	if _, err := os.Stat(orbitDir); !os.IsNotExist(err) {
		t.Fatalf(".orbit directory already exists")
	}

	cmd := newInitCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err = cmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Check .orbit directory was created
	if _, err := os.Stat(orbitDir); os.IsNotExist(err) {
		t.Errorf(".orbit directory was not created")
	}
}
