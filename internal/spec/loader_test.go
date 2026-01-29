package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidate_ValidFile(t *testing.T) {
	path := filepath.Join("testdata", "valid.md")

	spec, err := Validate([]string{path})
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	if spec == nil {
		t.Fatal("Validate() returned nil spec")
	}

	if len(spec.FilePaths) != 1 {
		t.Errorf("len(spec.FilePaths) = %d, want 1", len(spec.FilePaths))
	}

	// Should be absolute path
	if !filepath.IsAbs(spec.FilePaths[0]) {
		t.Errorf("spec.FilePaths[0] = %q, want absolute path", spec.FilePaths[0])
	}

	// Verify checksum is a valid SHA256 hex string (64 characters)
	if len(spec.Checksum) != 64 {
		t.Errorf("spec.Checksum length = %d, want 64", len(spec.Checksum))
	}
}

func TestValidate_FileNotFound(t *testing.T) {
	_, err := Validate([]string{"nonexistent/file.md"})

	if err == nil {
		t.Fatal("Validate() error = nil, want error for non-existent file")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Validate() error does not mention 'not found', got: %v", err)
	}
}

func TestValidate_Directory(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := Validate([]string{tmpDir})

	if err == nil {
		t.Fatal("Validate() error = nil, want error for directory")
	}

	if !strings.Contains(err.Error(), "directory") {
		t.Errorf("Validate() error does not mention 'directory', got: %v", err)
	}
}

func TestValidate_EmptyPaths(t *testing.T) {
	_, err := Validate([]string{})
	if err == nil {
		t.Fatal("Validate() error = nil, want error for empty paths")
	}
}

func TestValidate_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "spec1.md")
	file2 := filepath.Join(tmpDir, "spec2.md")

	if err := os.WriteFile(file1, []byte("# Spec 1"), 0644); err != nil {
		t.Fatalf("failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("# Spec 2"), 0644); err != nil {
		t.Fatalf("failed to create file2: %v", err)
	}

	spec, err := Validate([]string{file1, file2})
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	if len(spec.FilePaths) != 2 {
		t.Errorf("len(spec.FilePaths) = %d, want 2", len(spec.FilePaths))
	}
}

func TestValidate_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "exists.md")
	file2 := filepath.Join(tmpDir, "missing.md")

	if err := os.WriteFile(file1, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	_, err := Validate([]string{file1, file2})
	if err == nil {
		t.Fatal("Validate() error = nil, want error for missing file")
	}
}

func TestBuildPrompt_SingleFile(t *testing.T) {
	spec := &Spec{
		FilePaths: []string{"/path/to/spec.md"},
	}

	prompt := spec.BuildPrompt()

	if !strings.Contains(prompt, "/path/to/spec.md") {
		t.Error("BuildPrompt() missing file path")
	}
	if strings.Contains(prompt, "files") {
		t.Error("BuildPrompt() should say 'file' not 'files' for single file")
	}
}

func TestBuildPrompt_MultipleFiles(t *testing.T) {
	spec := &Spec{
		FilePaths: []string{"/path/to/spec1.md", "/path/to/spec2.md"},
	}

	prompt := spec.BuildPrompt()

	if !strings.Contains(prompt, "/path/to/spec1.md") {
		t.Error("BuildPrompt() missing first file path")
	}
	if !strings.Contains(prompt, "/path/to/spec2.md") {
		t.Error("BuildPrompt() missing second file path")
	}
	if !strings.Contains(prompt, "files") {
		t.Error("BuildPrompt() should say 'files' for multiple files")
	}
}

func TestBuildSystemPrompt_ContainsKeyContent(t *testing.T) {
	// Set completion promise and notes file for the test
	CompletionPromise = "<promise>COMPLETE</promise>"
	NotesFile = ".orbital/notes.md"
	defer func() {
		CompletionPromise = ""
		NotesFile = ".orbital/notes.md"
	}()

	systemPrompt := BuildSystemPrompt()

	// System prompt should contain key autonomous loop concepts
	if !strings.Contains(systemPrompt, "Autonomous Loop") {
		t.Error("BuildSystemPrompt() missing 'Autonomous Loop' heading")
	}
	if !strings.Contains(systemPrompt, "One Task Per Iteration") {
		t.Error("BuildSystemPrompt() missing 'One Task Per Iteration' principle")
	}
	if !strings.Contains(systemPrompt, "conventional commits") {
		t.Error("BuildSystemPrompt() missing conventional commits reference")
	}
}

func TestBuildSystemPrompt_SubstitutesPromise(t *testing.T) {
	CompletionPromise = "<test>DONE</test>"
	defer func() { CompletionPromise = "" }()

	systemPrompt := BuildSystemPrompt()

	if !strings.Contains(systemPrompt, "<test>DONE</test>") {
		t.Error("BuildSystemPrompt() should contain the completion promise")
	}
	if strings.Contains(systemPrompt, "{{promise}}") {
		t.Error("BuildSystemPrompt() should substitute {{promise}} placeholder")
	}
}

func TestBuildSystemPrompt_SubstitutesNotesFile(t *testing.T) {
	NotesFile = "custom/notes.md"
	defer func() { NotesFile = ".orbital/notes.md" }()

	systemPrompt := BuildSystemPrompt()

	if !strings.Contains(systemPrompt, "custom/notes.md") {
		t.Error("BuildSystemPrompt() should contain the notes file path")
	}
	if strings.Contains(systemPrompt, "{{notes_file}}") {
		t.Error("BuildSystemPrompt() should substitute {{notes_file}} placeholder")
	}
}

func TestSpec_StructFields(t *testing.T) {
	spec := Spec{
		FilePaths: []string{"/path/to/file.md"},
		Checksum:  "abc123",
	}

	if len(spec.FilePaths) != 1 || spec.FilePaths[0] != "/path/to/file.md" {
		t.Errorf("spec.FilePaths = %v, want [/path/to/file.md]", spec.FilePaths)
	}
	if spec.Checksum != "abc123" {
		t.Errorf("spec.Checksum = %q, want %q", spec.Checksum, "abc123")
	}
}

func TestBuildVerificationPrompt_SingleFile(t *testing.T) {
	prompt := BuildVerificationPrompt([]string{"/path/to/spec.md"})

	if !strings.Contains(prompt, "/path/to/spec.md") {
		t.Error("BuildVerificationPrompt() missing file path")
	}
	if !strings.Contains(prompt, "VERIFIED") {
		t.Error("BuildVerificationPrompt() missing VERIFIED format instruction")
	}
	if !strings.Contains(prompt, "INCOMPLETE") {
		t.Error("BuildVerificationPrompt() missing INCOMPLETE format instruction")
	}
	if strings.Contains(prompt, "{{files}}") {
		t.Error("BuildVerificationPrompt() should substitute {{files}} placeholder")
	}
}

func TestBuildVerificationPrompt_MultipleFiles(t *testing.T) {
	prompt := BuildVerificationPrompt([]string{"/path/to/spec1.md", "/path/to/spec2.md"})

	if !strings.Contains(prompt, "/path/to/spec1.md") {
		t.Error("BuildVerificationPrompt() missing first file path")
	}
	if !strings.Contains(prompt, "/path/to/spec2.md") {
		t.Error("BuildVerificationPrompt() missing second file path")
	}
	if !strings.Contains(prompt, "- /path/to/spec1.md") {
		t.Error("BuildVerificationPrompt() should format files as list items")
	}
}

func TestBuildVerificationPrompt_ContainsCheckboxInstructions(t *testing.T) {
	prompt := BuildVerificationPrompt([]string{"/path/to/spec.md"})

	if !strings.Contains(prompt, "[ ]") {
		t.Error("BuildVerificationPrompt() should mention unchecked checkbox pattern")
	}
	if !strings.Contains(prompt, "[x]") {
		t.Error("BuildVerificationPrompt() should mention checked checkbox pattern")
	}
	if !strings.Contains(prompt, "0 unchecked") {
		t.Error("BuildVerificationPrompt() should specify VERIFIED format with 0 unchecked")
	}
}

func TestVerificationPrompt_ContainsRequiredElements(t *testing.T) {
	// Test the constant directly
	if !strings.Contains(VerificationPrompt, "{{files}}") {
		t.Error("VerificationPrompt should contain {{files}} placeholder")
	}
	if !strings.Contains(VerificationPrompt, "VERIFIED") {
		t.Error("VerificationPrompt should contain VERIFIED keyword")
	}
	if !strings.Contains(VerificationPrompt, "INCOMPLETE") {
		t.Error("VerificationPrompt should contain INCOMPLETE keyword")
	}
}
