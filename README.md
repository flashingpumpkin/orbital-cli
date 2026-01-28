# Orbital CLI

A Go CLI tool that implements the Ralph Wiggum method for autonomous Claude Code iteration.

## Development Notice

This project was developed entirely through AI-driven development using Claude Code. No human has written, reviewed, or directly modified any code in this repository. Every line of Go, every test, and every piece of documentation was generated through natural language conversation with Claude.

This serves as both a practical tool and an experiment in autonomous software development.

**Warning:** Orbital runs Claude Code with `--dangerously-skip-permissions`, which bypasses all interactive approval prompts. Claude will execute shell commands, modify files, and make network requests without asking for confirmation. Only run this tool in environments where you accept these risks.

## What is the Ralph Wiggum Method?

The Ralph Wiggum method is an iterative AI development technique where a prompt is repeatedly fed to Claude Code until completion criteria are met. Named after the Simpsons character's optimistic persistence ("I'm learnding!"), it embodies:

- **Iteration beats perfection**: refine through loops, not perfect first attempts
- **Failures are data**: use failures to inform the next iteration
- **Persistence wins**: let the loop handle retries automatically

Each iteration builds on the previous work since files remain modified on disk. Claude sees its own previous changes and can improve upon them.

## Installation

```bash
go install github.com/flashingpumpkin/orbital-cli/cmd/orbital@latest
```

Or build from source:

```bash
git clone https://github.com/flashingpumpkin/orbital-cli.git
cd orbital-cli
go build -o orbital ./cmd/orbital
```

## Prerequisites

- Go 1.24 or later
- Claude CLI installed and in PATH (`claude`)
- Valid Anthropic API credentials configured

## Usage

```bash
orbital <spec-file> [--context <file>]... [--notes <file>] [flags]
```

### Examples

```bash
# Basic usage with TUI (default in interactive terminals)
orbital ./spec.md

# With additional context files
orbital ./spec.md --context ./docs/architecture.md --context ./docs/api.md

# Use a workflow preset
orbital ./spec.md --workflow reviewed

# TDD workflow with red-green-refactor cycle
orbital ./spec.md --workflow tdd

# Autonomous workflow with self-directed task selection
orbital ./spec.md --workflow autonomous

# Worktree isolation mode (work in isolated git worktree)
orbital ./spec.md --worktree

# Minimal output mode (no TUI)
orbital ./spec.md --minimal

# Fast workflow for maximum throughput
orbital ./spec.md --workflow fast
```

### Subcommands

| Command | Description |
|---------|-------------|
| `orbital init` | Create a default configuration file |
| `orbital status` | Display current session state and active files |
| `orbital continue` | Resume a previously interrupted session |

#### Session Resume

If orbital is interrupted (Ctrl+C or terminal closed), you can resume:

```bash
orbital continue
```

State is stored in `.orbital/state/` and automatically cleaned up on successful completion.

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--context` | | | Additional context file (can be repeated) |
| `--notes` | | auto | Path to notes file for cross-iteration context |
| `--iterations` | `-n` | 50 | Maximum iterations before stopping |
| `--promise` | `-p` | `<promise>COMPLETE</promise>` | Completion promise string |
| `--model` | `-m` | `opus` | Claude model for execution |
| `--checker-model` | | `haiku` | Claude model for completion checking |
| `--budget` | `-b` | 100.00 | Maximum USD to spend |
| `--working-dir` | `-d` | `.` | Working directory |
| `--config` | `-c` | `.orbital/config.toml` | Path to config file |
| `--workflow` | | `spec-driven` | Workflow preset (fast, spec-driven, reviewed, tdd) |
| `--worktree` | | false | Enable worktree isolation mode |
| `--worktree-name` | | | Override worktree name |
| `--setup-model` | | `haiku` | Model for worktree setup phase |
| `--merge-model` | | `haiku` | Model for worktree merge phase |
| `--minimal` | | false | Use minimal output mode (no TUI) |
| `--quiet` | `-q` | false | Suppress verbose output |
| `--debug` | | false | Stream raw JSON output |
| `--dry-run` | | false | Show what would be executed |
| `--session-id` | `-s` | | Use specific session ID |
| `--timeout` | `-t` | 5m | Timeout for all workflow steps (overrides step-level timeouts) |
| `--max-turns` | | 0 | Max agentic turns per iteration (0 = unlimited) |
| `--system-prompt` | | | Custom system prompt |
| `--agents` | | | JSON object defining custom agents |

## Workflow Presets

Orbital uses workflow steps for all execution. Each step has a prompt and an optional timeout (default: 5 minutes). If a step times out, it retries once with a continuation prompt before moving to the next iteration.

| Preset | Description |
|--------|-------------|
| `spec-driven` | Single implement step with completion check (default) |
| `fast` | Maximise work per iteration with review gate |
| `reviewed` | Implement with review gate before completion |
| `tdd` | Red-green-refactor cycle with review gate |
| `autonomous` | Self-directed task selection with fix step and review gate |

### TDD Workflow

The TDD workflow follows the red-green-refactor cycle:

1. **red**: Write a failing test for the next requirement
2. **green**: Write minimal code to make the test pass
3. **refactor**: Improve the code while keeping tests green
4. **review**: Gate step that validates the cycle (PASS/FAIL)

If the review gate fails, the workflow returns to the refactor step.

## Worktree Isolation Mode

Worktree mode isolates work in a git worktree, keeping the main branch clean until completion:

```bash
orbital ./spec.md --worktree
```

The workflow:
1. **Setup phase**: Claude analyses the spec and creates a named worktree with a feature branch
2. **Execution**: All work happens in the isolated worktree
3. **Merge phase**: On completion, changes are merged back to the original branch
4. **Cleanup**: Worktree and branch are removed after successful merge

If interrupted, the worktree is preserved and can be resumed with `orbital continue`.

## Terminal UI

Orbital includes a Bubbletea-based terminal UI that displays:

- Session information (spec files, notes file, state file)
- Progress (iteration count, budget, tokens)
- Workflow step progress (for multi-step workflows)
- Worktree status (when in worktree mode)
- Live output from Claude

The TUI is enabled by default in interactive terminals. Disable it with `--minimal` or `--quiet`.

## Configuration File

Orbital can be configured via a TOML file at `.orbital/config.toml`:

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

# Custom agents (optional)
[agents.reviewer]
description = "Code review specialist"
prompt = "You are a code reviewer."
```

### Step Configuration

| Field | Description |
|-------|-------------|
| `name` | Unique step identifier (required) |
| `prompt` | Prompt template with placeholders (required) |
| `timeout` | Step timeout duration (default: 5m) |
| `gate` | If true, step must output `<gate>PASS</gate>` or `<gate>FAIL</gate>` |
| `on_fail` | Step to jump to when gate fails |
| `deferred` | If true, step only runs when reached via `on_fail` |

### Template Placeholders

| Placeholder | Description |
|-------------|-------------|
| `{{files}}` | List of all file paths (spec + context) |
| `{{spec_file}}` | Primary spec file path |
| `{{context_files}}` | List of context file paths |
| `{{notes_file}}` | Path to notes file |
| `{{timeout}}` | Step timeout as human-readable text (e.g., "5 minutes") |
| `{{plural}}` | "s" if multiple files, empty otherwise |
| `{{promise}}` | Completion promise string |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success (completion promise detected) |
| 1 | Max iterations reached |
| 2 | Budget exceeded |
| 3 | Timeout |
| 4 | Other error |
| 130 | Interrupted (SIGINT/Ctrl+C) |

## Writing Spec Files

A spec file is a markdown file containing the task description. Include clear completion criteria.

### Example Spec

```markdown
# Task: Build REST API

Build a REST API for todo management.

## Requirements
- [ ] CRUD operations for todos
- [ ] Input validation
- [ ] Unit tests with >80% coverage
- [ ] OpenAPI documentation

## Completion Criteria
When all requirements are checked off and tests pass, output:
<promise>COMPLETE</promise>
```

### Best Practices

1. **Use checkboxes**: Mark requirements with `- [ ]` so Claude can check them off as `- [x]`
2. **Clear completion criteria**: Tell Claude exactly when to output the promise
3. **Incremental goals**: Break large tasks into phases
4. **Self-correction instructions**: Include debugging steps
5. **Escape hatches**: Always set `--iterations` as a safety net

## Architecture

```
orbital/
├── cmd/orbital/           # CLI entry point
│   ├── main.go
│   ├── root.go            # Main command and flags
│   ├── init.go            # orbital init subcommand
│   ├── status.go          # orbital status subcommand
│   ├── continue.go        # orbital continue subcommand
│   └── signal.go          # Graceful shutdown
├── internal/
│   ├── config/            # Configuration parsing and validation
│   ├── spec/              # Spec file loading and prompt building
│   ├── state/             # Session state and queue management
│   ├── completion/        # Promise string detection
│   ├── output/            # Stream parsing and formatting
│   ├── executor/          # Claude CLI process management
│   ├── loop/              # Main iteration controller
│   ├── workflow/          # Multi-step workflow engine
│   ├── worktree/          # Git worktree isolation
│   ├── tasks/             # Task tracking (TodoWrite)
│   └── tui/               # Bubbletea terminal UI
├── docs/
│   ├── plans/             # Tech specs and user stories
│   ├── notes/             # Session notes
│   └── decisions/         # Architecture decision records
├── go.mod
└── go.sum
```

## How It Works

1. **Load spec**: Read the task specification and context files
2. **Initialise**: Set up iteration counter, budget tracking, session state, and TUI
3. **Execute workflow steps**: Each step runs with its own timeout (default 5 minutes)
   - On timeout: retry once with continuation prompt ("continue from where you left off")
   - On second timeout: move to next iteration
4. **Parse output**: Extract text, tokens, and costs from Claude's stream-json output
5. **Check gates**: For gate steps, check for `<gate>PASS</gate>` or `<gate>FAIL</gate>`
   - On PASS: continue to next step
   - On FAIL: jump to `on_fail` step (or retry)
6. **Verify completion**: Run verification to check all spec items are complete
7. **Handle queue**: Process any dynamically added spec files
8. **Repeat or exit**: Continue until verification passes or limits reached

## Development

```bash
# Run lint and tests, then build (recommended)
make

# Build only
make build

# Run linter
make lint

# Run tests
make test

# Install locally
make install
```

## References

- [Ralph Wiggum Plugin](https://github.com/anthropics/claude-code/blob/main/plugins/ralph-wiggum/README.md): Original Anthropic plugin
- [Claude Code CLI Reference](https://code.claude.com/docs/en/cli-reference): Official CLI documentation

## Licence

MIT
