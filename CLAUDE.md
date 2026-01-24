# CLAUDE.md

This file provides context for Claude Code when working on this project.

## Project Overview

Orbital CLI is a Go tool that implements the "Ralph Wiggum method" for autonomous Claude Code iteration. It runs Claude Code in a loop, monitoring output for completion markers, with support for multi-step workflows, git worktree isolation, and a terminal UI.

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
│   │   └── file.go              # TOML file config loading
│   ├── spec/                    # Spec file loading and prompt building
│   │   ├── spec.go              # Spec struct and validation
│   │   └── prompt.go            # Prompt template rendering
│   ├── state/                   # Session state persistence
│   │   ├── state.go             # State struct and operations
│   │   └── queue.go             # Dynamic file queue management
│   ├── completion/              # Promise string detection
│   │   └── detector.go          # Completion marker matching
│   ├── output/                  # Stream parsing and formatting
│   │   ├── parser.go            # Claude stream-json parsing
│   │   ├── formatter.go         # Coloured terminal output
│   │   └── stream_processor.go  # Real-time stream processing
│   ├── executor/                # Claude CLI process management
│   │   └── executor.go          # Process spawning and output capture
│   ├── loop/                    # Main iteration controller
│   │   ├── controller.go        # Loop orchestration
│   │   └── verification.go      # Spec verification logic
│   ├── workflow/                # Multi-step workflow engine
│   │   ├── workflow.go          # Workflow and Step structs
│   │   ├── presets.go           # Built-in workflow presets
│   │   └── runner.go            # Workflow execution
│   ├── worktree/                # Git worktree isolation
│   │   ├── setup.go             # Worktree creation phase
│   │   ├── merge.go             # Merge back to main phase
│   │   ├── cleanup.go           # Worktree removal
│   │   └── state.go             # Worktree state persistence
│   ├── tasks/                   # Task tracking
│   │   └── tracker.go           # TodoWrite task management
│   └── tui/                     # Bubbletea terminal UI
│       ├── model.go             # TUI model and update logic
│       ├── view.go              # TUI rendering
│       ├── bridge.go            # Stream-to-TUI adapter
│       └── layout.go            # Panel layout management
├── docs/
│   ├── plans/                   # Tech specs and user stories
│   ├── notes/                   # Session notes
│   └── decisions/               # Architecture decision records
├── go.mod                       # Module: github.com/flashingpumpkin/orbital
└── go.sum
```

## Key Concepts

### The Loop

The core loop in `internal/loop/controller.go`:
1. Execute Claude CLI with the spec content as prompt
2. Parse stream-json output for tokens, cost, text
3. Check if completion promise exists in output
4. If found, run verification to confirm all items complete
5. If budget exceeded or max iterations reached, exit with error
6. Otherwise, repeat

### Workflow Engine

Multi-step workflows in `internal/workflow/`:
- **Steps**: Named execution steps with prompts
- **Gates**: Steps that output `<gate>PASS</gate>` or `<gate>FAIL</gate>`
- **OnFail**: Gate failure redirects to a specified step
- **Presets**: fast, spec-driven (default), reviewed, tdd

### Worktree Isolation

Git worktree mode in `internal/worktree/`:
1. **Setup**: Claude names the feature, creates worktree and branch
2. **Execute**: All iterations run in the isolated worktree
3. **Merge**: On completion, merge branch back to original
4. **Cleanup**: Remove worktree and branch

### Terminal UI

Bubbletea-based TUI in `internal/tui/`:
- Fixed panels: session info, progress, worktree status
- Scrolling output panel with Claude's responses
- Real-time token/cost tracking
- Workflow step progress display

### Completion Promise

Default: `<promise>COMPLETE</promise>`

The spec file instructs Claude to output this string when done. The detector does case-sensitive exact matching.

### Verification

After promise detection, verification runs using the checker model to confirm all spec items are checked off (`- [x]`). This prevents premature completion.

## Build Commands

```bash
# Build
go build ./cmd/orbital

# Test all packages
go test ./...

# Test with coverage
go test -cover ./...

# Install to GOPATH/bin
go install ./cmd/orbital
```

## Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/BurntSushi/toml` - TOML configuration parsing
- `github.com/fatih/color` - Coloured terminal output
- `github.com/briandowns/spinner` - Progress spinner
- `github.com/charmbracelet/bubbletea` - Terminal UI framework
- `github.com/charmbracelet/lipgloss` - Terminal styling

## Testing Patterns

All packages use table-driven tests. Mock interfaces are defined for dependency injection:
- `loop.ExecutorInterface` - Mock executor for loop tests
- `workflow.StepExecutor` - Mock executor for workflow tests
- `worktree.Executor` - Mock executor for worktree tests

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
MaxIterations:     50
CompletionPromise: "<promise>COMPLETE</promise>"
Model:             "opus"
CheckerModel:      "haiku"
MaxBudget:         100.00
WorkingDir:        "."
IterationTimeout:  30 * time.Minute
```

## Configuration File

Orbital supports an optional TOML config file at `.orbital/config.toml`:

```toml
# Custom prompt template
prompt = """
Implement the stories in {{files}}
When done, output <promise>COMPLETE</promise>
"""

# Custom workflow
[workflow]
name = "custom"
[[workflow.steps]]
name = "implement"
prompt = "..."

# Custom agents
[agents.reviewer]
description = "Code reviewer"
prompt = "You review code."
```

Placeholders:
- `{{files}}` - List of spec file paths
- `{{plural}}` - "s" if multiple files
- `{{promise}}` - Completion promise string

## Workflow Presets

| Preset | Steps | Description |
|--------|-------|-------------|
| `fast` | implement, review (gate) | Maximise work per iteration |
| `spec-driven` | implement | Single step, completion check (default) |
| `reviewed` | implement, review (gate) | Review gate before completion |
| `tdd` | red, green, refactor, review (gate) | TDD cycle |

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
