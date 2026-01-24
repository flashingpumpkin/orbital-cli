// Package spec provides functionality for loading and parsing specification files.
package spec

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Spec represents one or more specification files to be processed.
type Spec struct {
	// FilePaths contains all file paths to process.
	FilePaths []string

	// Checksum is a hash of the file paths for change detection.
	Checksum string
}

// Validate checks that the given file paths exist and are readable.
// Returns an error if any file is not found or cannot be read.
func Validate(paths []string) (*Spec, error) {
	if len(paths) == 0 {
		return nil, errors.New("at least one spec file is required")
	}

	// Convert to absolute paths and validate
	absPaths := make([]string, 0, len(paths))
	for _, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("invalid path %s: %w", path, err)
		}

		info, err := os.Stat(absPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("spec file not found: %s", path)
			}
			return nil, fmt.Errorf("cannot access spec file %s: %w", path, err)
		}

		if info.IsDir() {
			return nil, fmt.Errorf("spec path is a directory: %s", path)
		}

		absPaths = append(absPaths, absPath)
	}

	// Generate checksum from paths
	checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(absPaths, "\n"))))

	return &Spec{
		FilePaths: absPaths,
		Checksum:  checksum,
	}, nil
}

// PromptTemplate holds the template for building user prompts.
// Can be set from config file.
var PromptTemplate string

// SystemPromptTemplate holds the template for the system prompt.
// Can be set from config file.
var SystemPromptTemplate string

// CompletionPromise holds the promise string to include in prompts.
var CompletionPromise string

// NotesFile holds the path to the notes file for cross-iteration context.
// Default: .orbit/notes.md
var NotesFile = ".orbit/notes.md"

// DefaultSystemPrompt contains methodology and rules appended to Claude's system prompt.
// This stays constant across iterations.
const DefaultSystemPrompt = `# Autonomous Loop

You are operating in an autonomous loop. Each iteration, you receive this prompt with fresh context. You work until you output the completion promise.

This technique uses autonomous loops to drive you toward completion through persistent, simple, repetitive action. It works best for tasks with clear, verifiable criteria: refactoring, test coverage, documentation. It is not suited for creative, ambiguous, or security-sensitive work.

## Tenets

**Fresh Context Is Reliability** — Each iteration clears your context. Re-read specs, plan, and code every cycle. Errors compound when you rely on stale assumptions.

**Backpressure Over Prescription** — You are not told exactly how to code. Instead, gates (tests, linting, type-checking) reject bad work automatically. Adapt and correct yourself.

**The Plan Is Disposable** — If a plan drifts, regenerate it. Cheaper than fixing it. Keep implementation plans, but do not treat them as sacred.

**Disk Is State, Git Is Memory** — Files on disk and git commits are your handoff mechanism between iterations.

**Steer With Signals, Not Scripts** — Failed tests guide you. The codebase is your instruction manual.

**Work Autonomously** — You work without intervention. The operator sits on the loop, not in it.

## Principles

**One Task Per Iteration** — Complete one item per iteration. No exceptions. Small changes prevent context rot.

**Feedback Loops Are Mandatory** — Tests, type checking, and linting must pass before you mark a task complete.

**Fail Predictably** — It is better to fail in a predictable way that can be fixed via prompting than to succeed in unpredictable, unmaintainable ways.

## Notes File

Maintain notes in: ` + "`{{notes_file}}`" + `

This file captures observations, blockers, and decisions across iterations. Create it if it does not exist. Do not track task status here; that belongs in the spec/stories file.

Use it for:
- Blockers or issues you encountered
- Decisions that affect future work
- Anything the next iteration needs to know

## Workflow

Each iteration:

1. Read the spec/stories file. Find the next pending item (first ` + "`[ ]`" + ` checkbox).
2. Read ` + "`{{notes_file}}`" + ` for context from previous iterations.
3. Plan and implement that single item.
4. Run verification: tests, lint, typecheck, build.
5. If verification passes, mark the item ` + "`[x]`" + ` in the spec/stories file.
6. Update ` + "`{{notes_file}}`" + ` with relevant observations.
7. Commit your changes.
8. Exit.

If verification fails, debug and fix before marking complete. If you cannot resolve an issue after reasonable effort, document the blocker in ` + "`{{notes_file}}`" + ` and move on.

## Commits

Use conventional commits:

` + "```" + `
<type>(<scope>): <description>
` + "```" + `

Types: ` + "`feat`" + `, ` + "`fix`" + `, ` + "`refactor`" + `, ` + "`test`" + `, ` + "`docs`" + `, ` + "`chore`" + `

Stage your changes, the spec/stories file, and ` + "`{{notes_file}}`" + `. Do not stage unrelated files.

## Stop Condition

Do not output the completion promise until:

1. Every item in the spec/stories file is marked ` + "`[x]`" + `
2. All verification checks pass
3. All changes are committed

Before outputting the promise:

1. Read the spec/stories file one final time
2. Search for any remaining ` + "`[ ]`" + ` unchecked boxes
3. Run verification one more time
4. If any unchecked box exists or any check fails, do not output the promise

Only after verification confirms zero unchecked boxes and all checks pass, output:

` + "```" + `
{{promise}}
` + "```" + `

Output nothing after the promise.

## If You Get Stuck

Document the blocker in ` + "`{{notes_file}}`" + ` and exit. Let the next iteration try a different approach.`

// DefaultPrompt is the default user prompt template (task-specific, varies per iteration).
const DefaultPrompt = `Implement the next pending user story from the following spec file{{plural}}:

{{files}}`

// VerificationPrompt is the prompt template for verifying completion.
// Used by the verification step to check if all checkboxes are complete.
const VerificationPrompt = `Read the following spec file(s) and count the checkboxes:

{{files}}

Count all checkbox patterns:
- Unchecked: [ ] (space between brackets)
- Checked: [x] or [X] (x or X between brackets)

Respond with EXACTLY one of these formats (nothing else):
- If zero unchecked boxes: VERIFIED: 0 unchecked, N checked
- If any unchecked boxes: INCOMPLETE: N unchecked, M checked

Replace N and M with the actual counts.`

// BuildPrompt generates the prompt to send to Claude CLI.
// Uses PromptTemplate if set, otherwise uses default template.
func (s *Spec) BuildPrompt() string {
	template := PromptTemplate
	if template == "" {
		template = DefaultPrompt
	}

	// Use template with placeholders
	plural := ""
	if len(s.FilePaths) > 1 {
		plural = "s"
	}
	result := strings.ReplaceAll(template, "{{plural}}", plural)

	var fileList strings.Builder
	for _, path := range s.FilePaths {
		fileList.WriteString("- ")
		fileList.WriteString(path)
		fileList.WriteString("\n")
	}
	result = strings.ReplaceAll(result, "{{files}}", strings.TrimSuffix(fileList.String(), "\n"))

	// Replace promise placeholder
	if CompletionPromise != "" {
		result = strings.ReplaceAll(result, "{{promise}}", CompletionPromise)
	}

	return result
}

// BuildSystemPrompt generates the system prompt to append via --append-system-prompt.
// Uses SystemPromptTemplate if set, otherwise uses default system prompt.
func BuildSystemPrompt() string {
	template := SystemPromptTemplate
	if template == "" {
		template = DefaultSystemPrompt
	}

	// Replace placeholders
	result := template
	if CompletionPromise != "" {
		result = strings.ReplaceAll(result, "{{promise}}", CompletionPromise)
	}
	if NotesFile != "" {
		result = strings.ReplaceAll(result, "{{notes_file}}", NotesFile)
	}

	return result
}

// BuildVerificationPrompt generates the prompt for the verification step.
// Takes a list of spec file paths and returns a prompt instructing Claude
// to count checkboxes and report completion status.
func BuildVerificationPrompt(files []string) string {
	var fileList strings.Builder
	for _, path := range files {
		fileList.WriteString("- ")
		fileList.WriteString(path)
		fileList.WriteString("\n")
	}
	return strings.ReplaceAll(VerificationPrompt, "{{files}}", strings.TrimSuffix(fileList.String(), "\n"))
}

// Load is deprecated - use Validate instead.
// Kept for backward compatibility.
func Load(path string) (*Spec, error) {
	return Validate([]string{path})
}

// LoadMultiple is deprecated - use Validate instead.
// Kept for backward compatibility.
func LoadMultiple(paths []string) (*Spec, error) {
	return Validate(paths)
}
