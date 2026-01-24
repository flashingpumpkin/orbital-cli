# Orbital CLI

> **Note:** This entire project was built through AI-assisted development. No line of code was manually written or reviewed by a human.

A Go CLI tool that implements the Ralph Wiggum method for autonomous Claude Code iteration.

## What is the Ralph Wiggum Method?

The Ralph Wiggum method is an iterative AI development technique where a prompt is repeatedly fed to Claude Code until completion criteria are met. Named after the Simpsons character's optimistic persistence ("I'm learnding!"), it embodies:

- **Iteration beats perfection** - refine through loops, not perfect first attempts
- **Failures are data** - use failures to inform the next iteration
- **Persistence wins** - let the loop handle retries automatically

Each iteration builds on the previous work since files remain modified on disk. Claude sees its own previous changes and can improve upon them.

## Installation

```bash
go install github.com/flashingpumpkin/orbital/cmd/orbital@latest
```

Or build from source:

```bash
git clone https://github.com/flashingpumpkin/orbital.git
cd orbital
go build -o orbital ./cmd/orbital
```

To install from a cloned repository:

```bash
# From within the repository directory
go install ./cmd/orbital

# Or build and copy to a specific location
go build -o /usr/local/bin/orbital ./cmd/orbital
```

## Prerequisites

- Go 1.21 or later
- Claude CLI installed and in PATH (`claude`)
- Valid Anthropic API credentials configured

## Usage

```bash
orbital <spec-file> [--context <file>]... [--notes <file>] [flags]
```

### Examples

```bash
# Single spec file
orbital ./spec.md

# With additional context files
orbital ./spec.md --context ./docs/architecture.md --context ./docs/api.md

# With a notes file for cross-iteration context
orbital ./spec.md --notes ./notes.md

# Specify custom completion promise
orbital ./spec.md --promise "DONE"

# Quiet mode (suppress verbose output)
orbital ./spec.md --quiet

# Dry run to see what would happen
orbital ./spec.md --dry-run

# Use a different model for completion checking
orbital ./spec.md --checker-model sonnet
```

### Subcommands

| Command | Description |
|---------|-------------|
| `orbital status` | Display current session state and active files |
| `orbital continue` | Resume a previously interrupted session |
| `orbital init` | Create a default configuration file |

#### Session Resume

If orbit is interrupted (Ctrl+C or terminal closed), you can resume:

```bash
# Start a long-running task
orbital ./large-spec.md --iterations 100

# ... interrupted ...

# Resume from where you left off
orbital continue
```

State is stored in `.orbital/state/` and automatically cleaned up on successful completion.

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--context` | | | Additional context file (can be repeated) |
| `--notes` | | auto | Path to notes file for cross-iteration context |
| `--iterations` | `-n` | 50 | Maximum number of iterations before stopping |
| `--promise` | `-p` | `<promise>COMPLETE</promise>` | Completion promise string to detect success |
| `--model` | `-m` | `opus` | Claude model to use for execution |
| `--checker-model` | | `haiku` | Claude model to use for completion checking |
| `--budget` | `-b` | 100.00 | Maximum USD to spend on API calls |
| `--working-dir` | `-d` | `.` | Working directory for Claude Code |
| `--config` | `-c` | `.orbital/config.toml` | Path to config file |
| `--quiet` | `-q` | false | Suppress verbose output (verbose is default) |
| `--debug` | | false | Stream raw JSON output from Claude |
| `--dry-run` | | false | Show what would be executed without running |
| `--session-id` | `-s` | | Use specific session ID for continuity |
| `--timeout` | `-t` | 30m | Timeout per iteration |

### Configuration File

Orbit can be configured via a TOML file. By default, it looks for `.orbital/config.toml` in the working directory.

```toml
# .orbital/config.toml

# Custom prompt template
prompt = """
Read the spec file{{plural}}:
{{files}}

When ALL stories have [x] checked, output: {{promise}}
"""
```

#### Template Placeholders

| Placeholder | Description |
|-------------|-------------|
| `{{files}}` | List of spec file paths (formatted as `- /path/to/file`) |
| `{{plural}}` | "s" if multiple files, empty string otherwise |
| `{{promise}}` | The completion promise string (from `--promise` flag) |

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Completion promise detected (success) |
| 1 | Max iterations reached without completion |
| 2 | Budget exceeded |
| 3 | Timeout |
| 4 | Other error |
| 130 | Interrupted (SIGINT/Ctrl+C) |

## Writing Spec Files

A spec file is a markdown or text file containing the task description for Claude. Include clear completion criteria.

### Example Spec

```markdown
# Task: Build REST API

Build a REST API for todo management.

## Requirements
- CRUD operations for todos
- Input validation
- Unit tests with >80% coverage
- OpenAPI documentation

## Completion Criteria
When all requirements are met and tests pass, output:
<promise>COMPLETE</promise>
```

### Best Practices

1. **Clear completion criteria** - Tell Claude exactly when to output the promise
2. **Incremental goals** - Break large tasks into phases
3. **Self-correction instructions** - Include debugging steps
4. **Escape hatches** - Always set `--iterations` as a safety net

## Architecture

```
orbital/
├── cmd/orbital/       # CLI entry point
│   ├── main.go
│   ├── root.go          # Main command and flags
│   ├── init.go          # orbital init subcommand
│   ├── status.go        # orbital status subcommand
│   ├── continue.go      # orbital continue subcommand
│   └── signal.go        # Graceful shutdown
├── internal/
│   ├── config/          # Configuration parsing
│   ├── spec/            # Spec file loading
│   ├── state/           # Session state & queue management
│   ├── completion/      # Promise detection
│   ├── output/          # Stream parsing & formatting
│   ├── executor/        # Claude CLI orchestration
│   └── loop/            # Iteration controller
└── docs/plans/          # Tech spec & user stories
```

## How It Works

1. **Load spec** - Read the task specification from file(s)
2. **Initialise loop** - Set up iteration counter, budget tracking, create session state
3. **Execute Claude** - Run `claude -p --output-format stream-json` with the spec
4. **Parse output** - Extract text, tokens, and costs from stream
5. **Check completion** - Look for the promise string in output
6. **Repeat or exit** - Continue iterating until promise detected or limits reached

## Development

### Running Tests

```bash
go test ./...
```

### Building

```bash
go build ./cmd/orbital
```

## References

- [Ralph Wiggum Plugin](https://github.com/anthropics/claude-code/blob/main/plugins/ralph-wiggum/README.md) - Original Anthropic plugin
- [Claude Code CLI Reference](https://code.claude.com/docs/en/cli-reference) - Official CLI documentation

## License

MIT
