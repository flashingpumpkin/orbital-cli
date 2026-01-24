# CLAUDE.md

This file provides context for Claude Code when working on this project.

## Project Overview

Orbital CLI is a Go tool that implements the "Ralph Wiggum method" - an iterative development technique where Claude Code receives the same prompt repeatedly until a completion promise is detected in the output.

## Repository Structure

```
orbital/
├── cmd/orbital/             # CLI entry point
│   ├── main.go             # Calls rootCmd.Execute()
│   ├── root.go             # Cobra command, flags, main logic
│   └── signal.go           # SIGINT/SIGTERM handler
├── internal/
│   ├── config/             # Config struct, defaults, validation
│   ├── spec/               # Spec file loading (supports multiple files)
│   ├── completion/         # Promise string detection
│   ├── output/
│   │   ├── parser.go       # Claude stream-json parsing
│   │   └── formatter.go    # Coloured terminal output
│   ├── executor/           # Claude CLI process management
│   └── loop/               # Main iteration controller
├── docs/plans/             # Tech spec and user stories
├── go.mod                  # Module: github.com/flashingpumpkin/orbital
└── go.sum
```

## Key Concepts

### The Loop

The core loop in `internal/loop/controller.go`:
1. Execute Claude CLI with the spec content as prompt
2. Parse stream-json output for tokens, cost, text
3. Check if completion promise exists in output
4. If found, exit with success
5. If budget exceeded or max iterations reached, exit with error
6. Otherwise, repeat

### Completion Promise

Default: `<promise>COMPLETE</promise>`

The spec file should instruct Claude to output this exact string when the task is complete. The detector does case-sensitive exact matching.

### Claude CLI Invocation

```bash
claude -p \
  --output-format stream-json \
  --model <model> \
  --max-budget-usd <budget> \
  [--resume <session-id>] \
  "<prompt>"
```

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
- `github.com/spf13/viper` - Configuration (available but not heavily used)
- `github.com/fatih/color` - Coloured terminal output
- `github.com/briandowns/spinner` - Progress spinner

## Testing Patterns

All packages use table-driven tests. Mock interfaces are defined for dependency injection:
- `loop.ExecutorInterface` - Mock executor for loop tests

Test files are co-located with implementation: `foo.go` has `foo_test.go`.

## Exit Codes

| Code | Constant | Meaning |
|------|----------|---------|
| 0 | - | Success (promise detected) |
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
MaxBudget:         100.00
WorkingDir:        "."
IterationTimeout:  30 * time.Minute
```

## Configuration File

Orbital supports an optional TOML config file at `.orbital/config.toml` (or custom path via `--config`).

```toml
# Custom prompt template with placeholders
prompt = """
Implement the stories in {{files}}
When done, output <promise>COMPLETE</promise>
"""
```

Placeholders:
- `{{files}}` - List of spec file paths
- `{{plural}}` - "s" if multiple files

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
