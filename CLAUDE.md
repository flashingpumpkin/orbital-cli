# CLAUDE.md

This file provides context for Claude Code when working on this project.

## Project Overview

Orbital CLI is a Go tool that implements the "Ralph Wiggum method" for autonomous Claude Code iteration. It runs Claude Code in a loop, monitoring output for completion markers, with support for multi-step workflows and a terminal UI.

## Repository Structure

```
orbital/
├── cmd/orbital/                 # CLI entry point
│   ├── main.go                  # Calls rootCmd.Execute()
│   ├── root.go                  # Cobra command, flags, main orchestration
│   ├── init.go                  # orbital init subcommand
│   ├── status.go                # orbital status subcommand
│   ├── continue.go              # orbital continue subcommand
│   └── signal.go                # SIGINT/SIGTERM handler
├── internal/
│   ├── config/                  # Configuration parsing and validation
│   │   ├── config.go            # Main Config struct
│   │   ├── file.go              # TOML file config loading
│   │   └── agents.go            # Custom agent configuration
│   ├── spec/                    # Spec file loading and prompt building
│   │   ├── spec.go              # Spec struct and validation
│   │   └── loader.go            # Spec file loading
│   ├── state/                   # Session state persistence
│   │   └── state.go             # State struct and operations
│   ├── session/                 # Session management and discovery
│   │   ├── session.go           # Session struct and display
│   │   └── collector.go         # Session discovery and validation
│   ├── completion/              # Promise string detection
│   │   └── detector.go          # Completion marker matching
│   ├── output/                  # Stream parsing and formatting
│   │   ├── parser.go            # Claude stream-json parsing
│   │   ├── formatter.go         # Colored terminal output
│   │   └── stream.go            # Real-time stream processing
│   ├── executor/                # Claude CLI process management
│   │   └── executor.go          # Process spawning and output capture
│   ├── loop/                    # Main iteration controller
│   │   └── controller.go        # Loop orchestration
│   ├── workflow/                # Multi-step workflow engine
│   │   ├── workflow.go          # Workflow and Step structs
│   │   ├── presets.go           # Built-in workflow presets
│   │   ├── executor.go          # Runner and step execution with timeouts
│   │   └── gate.go              # Gate checking logic
│   ├── tasks/                   # Task tracking
│   │   └── tracker.go           # TodoWrite task management
│   ├── util/                    # Utility functions
│   └── tui/                     # Bubbletea terminal UI
│       ├── model.go             # TUI model and update logic
│       ├── view.go              # TUI rendering
│       ├── bridge.go            # Stream-to-TUI adapter
│       ├── layout.go            # Panel layout management
│       ├── themes.go            # Color theme support
│       ├── styles.go            # Lipgloss styles
│       ├── tasks.go             # Task display
│       ├── ringbuffer.go        # Output buffer management
│       ├── messages.go          # Bubbletea messages
│       ├── program.go           # Program initialization
│       └── selector/            # Session selector UI
│           ├── model.go         # Selector model
│           └── styles.go        # Selector styles
├── docs/
│   ├── plans/                   # Tech specs and user stories
│   ├── notes/                   # Session notes
│   └── specs/                   # Specification documents
├── go.mod                       # Module: github.com/flashingpumpkin/orbital-cli
└── go.sum
```

## Key Concepts

### The Loop

All execution uses workflow steps. Each iteration runs through the workflow's steps in sequence:
1. Execute each workflow step with its prompt (with per-step timeout, default 5 minutes)
2. Parse stream-json output for tokens, cost, text
3. For gate steps, check for `<gate>PASS</gate>` or `<gate>FAIL</gate>`
4. On timeout: retry once with continuation prompt, then fail
5. After all steps complete, run verification to confirm all spec items are done
6. If budget exceeded or max iterations reached, exit with error
7. Otherwise, repeat the workflow

### Workflow Engine

Multi-step workflows in `internal/workflow/`:
- **Steps**: Named execution steps with prompts and per-step timeouts
- **Timeout**: Each step has a timeout (default 5 minutes); on timeout, retries once with continuation prompt
- **Gates**: Steps that output `<gate>PASS</gate>` or `<gate>FAIL</gate>`
- **OnFail**: Gate failure redirects to a specified step
- **Deferred**: Steps marked deferred only run when reached via OnFail
- **Presets**: fast, spec-driven (default), reviewed, tdd, autonomous

### Terminal UI

Bubbletea-based TUI in `internal/tui/`:
- **Session information panel**: Displays spec files, notes file, state file
- **Progress panel**: Iteration count, workflow step progress, budget tracking
- **Multi-tab interface**: Switch between output and file content views
- **Session selector**: Interactive UI for resuming interrupted sessions
- **Theme support**: Auto-detect or manual theme selection (dark/light)
- **Real-time token/cost tracking**: Updates as Claude processes
- **Workflow step progress display**: Shows current step in multi-step workflows

### Completion Promise

Default: `<promise>COMPLETE</promise>`

The spec file instructs Claude to output this string when done. The detector does case-sensitive exact matching.

### Session Management

Session state in `internal/state/` and `internal/session/`:
- **State persistence**: All session state stored in `.orbital/state/`
- **Session discovery**: Find and validate resumable sessions
- **Interactive selector**: TUI for choosing which session to resume
- **State cleanup**: Automatic cleanup on successful completion
- **Session ID tracking**: Each Claude session gets a unique ID for resumption

Session state includes:
- Iteration count and budget spent
- Workflow state (current step, gate retries)
- File paths (spec, context, notes)
- Claude session ID for resumption

### Verification

After promise detection, verification runs using the checker model to confirm all spec items are checked off (`- [x]`). This prevents premature completion.

## Build Commands

```bash
# Run lint and tests, then build (recommended)
make

# Build only
make build

# Run linter
make lint

# Run tests
make test

# Run lint and tests
make check

# Install to GOPATH/bin
make install
```

## Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/BurntSushi/toml` - TOML configuration parsing
- `github.com/fatih/color` - Colored terminal output
- `github.com/briandowns/spinner` - Progress spinner
- `github.com/charmbracelet/bubbletea` - Terminal UI framework
- `github.com/charmbracelet/lipgloss` - Terminal styling
- `github.com/charmbracelet/bubbles` - TUI components (viewport)
- `golang.org/x/term` - Terminal detection and control

## Testing Patterns

All packages use table-driven tests. Mock interfaces are defined for dependency injection:
- `loop.Executor` - Mock executor for loop tests
- `workflow.StepExecutor` - Mock executor for workflow tests

Test files are co-located with implementation: `foo.go` has `foo_test.go`.

## Exit Codes

| Code | Constant | Meaning |
|------|----------|---------|
| 0 | - | Success (promise detected and verified) |
| 1 | `loop.ErrMaxIterationsReached` | Max iterations without completion |
| 2 | `loop.ErrBudgetExceeded` | Budget limit hit |
| 3 | `context.DeadlineExceeded` | Timeout |
| 4 | - | Other error |
| 130 | `context.Canceled` | User interrupt (Ctrl+C) |

## Configuration Defaults

```go
MaxIterations:      50
CompletionPromise:  "<promise>COMPLETE</promise>"
Model:              "opus"
CheckerModel:       "haiku"
MaxBudget:          100.00
WorkingDir:         "."
DefaultStepTimeout: 5 * time.Minute  // Per workflow step
```

## Configuration File

Orbital supports an optional TOML config file at `.orbital/config.toml`:

```toml
# Custom workflow
[workflow]
name = "custom"

[[workflow.steps]]
name = "implement"
timeout = "10m"  # Override default 5 minute timeout
prompt = """
Study the spec file: {{spec_file}}
Context files: {{context_files}}
Notes file: {{notes_file}}
Implement the next pending task.
"""

[[workflow.steps]]
name = "fix"
deferred = true  # Only runs when reached via on_fail
prompt = "Fix issues identified in the review."

[[workflow.steps]]
name = "review"
prompt = "Review the changes. Output <gate>PASS</gate> or <gate>FAIL</gate>"
gate = true
on_fail = "fix"

# Custom agents
[agents.reviewer]
description = "Code reviewer"
prompt = "You review code."
```

Template placeholders:
- `{{files}}` - List of all file paths (spec + context)
- `{{spec_file}}` - Primary spec file path
- `{{context_files}}` - List of context file paths
- `{{notes_file}}` - Path to notes file
- `{{timeout}}` - Step timeout as human-readable text (e.g., "5 minutes")
- `{{plural}}` - "s" if multiple files
- `{{promise}}` - Completion promise string

Built-in agents (always available via Task tool):
- `general-purpose` - Research, code exploration, multi-step tasks
- `security-reviewer` - Security vulnerabilities and attack vectors
- `design-reviewer` - Architecture, SOLID principles, coupling
- `logic-reviewer` - Bugs, edge cases, race conditions
- `error-reviewer` - Error handling and recovery patterns
- `data-reviewer` - Data handling, consistency, null safety

These are used in review gates for rigorous code review across multiple dimensions.

## Workflow Presets

| Preset | Steps | Description |
|--------|-------|-------------|
| `fast` | implement, review (gate) | Maximise work per iteration |
| `spec-driven` | implement | Single step, completion check (default) |
| `reviewed` | implement, review (gate) | Review gate before completion |
| `tdd` | red, green, refactor, review (gate) | TDD cycle |
| `autonomous` | implement, fix (deferred), review (gate) | Self-directed task selection with review gate |

## Claude CLI Invocation

```bash
claude -p \
  --output-format stream-json \
  --model <model> \
  --max-budget-usd <budget> \
  [--resume <session-id>] \
  [--max-turns <turns>] \
  [--append-system-prompt "<system-prompt>"] \
  [--agents '<agents-json>'] \
  "<prompt>"
```

## Adding New Features

1. Write tests first (TDD)
2. Implement in appropriate `internal/` package
3. Wire into `cmd/orbital/root.go` if CLI-facing
4. Update README.md if user-facing
5. Run `go test ./...` before committing

## Code Style

- Standard Go formatting (`gofmt`)
- Descriptive variable names
- Comments for exported functions
- Error wrapping with `fmt.Errorf("context: %w", err)`
- Table-driven tests with subtests
- Interface-based dependency injection for testability
