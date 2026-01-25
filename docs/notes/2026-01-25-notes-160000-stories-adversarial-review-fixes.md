# Notes: Adversarial Review Critical Fixes

## 2026-01-25

### Session Start

Working through adversarial review fixes from `docs/plans/2026-01-25-160000-stories-adversarial-review-fixes.md`.

### SEC-1: Make permission skip flag configurable

**Completed**

Implementation details:
1. Added `DangerouslySkipPermissions bool` to `config.Config` in `internal/config/config.go`
2. Added `Dangerous bool` to `FileConfig` in `internal/config/file.go` for TOML support
3. Added `--dangerous` CLI flag to root command (default: false) in `cmd/orbital/root.go`
4. Modified `executor.BuildArgs()` to conditionally include `--dangerously-skip-permissions` only when enabled
5. Added warning message to stderr when dangerous mode is enabled
6. Updated existing tests that assumed `--dangerously-skip-permissions` was always present
7. Added new tests: `TestBuildArgs_WithDangerousMode`, `TestBuildArgs_WithoutDangerousMode`, `TestLoadFileConfig_WithDangerous`, `TestLoadFileConfig_WithoutDangerous`

Breaking change: By default, `--dangerously-skip-permissions` is no longer passed to Claude CLI. Users must explicitly opt-in via `--dangerous` flag or `dangerous = true` in config.

## Code Review - Iteration 1

### Security
No issues. The implementation follows secure-by-default principles:
- Boolean flag cannot be exploited for injection
- Go's `exec.CommandContext()` uses `execve()` syscall with separate argv array (not shell interpolation)
- Config file requires local filesystem access (already implies code execution capability)
- CLI flag takes precedence over config file for safe mode

### Design
No issues. The implementation follows existing patterns:
- Single Responsibility maintained across config, file config, and executor
- Coupling is appropriate (executor depends only on config.Config)
- Naming is consistent (CLI `--dangerous`, TOML `dangerous`, struct `DangerouslySkipPermissions`)
- Precedence logic matches existing patterns for `agents` flag

### Logic
**ISSUES FOUND**

**BUG: Missing `DangerouslySkipPermissions` in `continue.go`**

File: `cmd/orbital/continue.go` lines 161-176

The `continue` command creates a `config.Config` struct but does not include the `DangerouslySkipPermissions` field. This means:
1. `--dangerous` CLI flag is ignored when using `orbital continue`
2. `dangerous = true` in config file is not applied
3. Users expecting consistent behaviour between `orbital <spec>` and `orbital continue` will find dangerous mode silently fails to activate

Missing code:
- Line 176 should include `DangerouslySkipPermissions: dangerous,`
- After line 201, needs the config file override logic
- After line 201, needs the warning message when dangerous mode is enabled

### Error Handling
No issues. Boolean flags have safe defaults, nil checks are in place, warning is output to stderr.

### Data Integrity
No issues. Boolean type provides inherent validation, nil check for `fileConfig` is present, precedence logic is correct.

### Verdict
**FAIL** - Logic review found a critical bug in `continue.go` where the `DangerouslySkipPermissions` field is not set, causing inconsistent behaviour between `orbital` and `orbital continue` commands.

## Code Review Feedback - Addressed

### BUG FIX: `DangerouslySkipPermissions` missing in continue.go

**Fixed** in this iteration.

Changes made to `cmd/orbital/continue.go`:
1. Added `DangerouslySkipPermissions: dangerous,` to the config struct (line 176)
2. Added config file override logic after fileConfig handling (lines 204-208)
3. Added warning message when dangerous mode is enabled (lines 210-213)

The fix mirrors the implementation in `root.go` exactly, ensuring consistent behaviour between `orbital <spec>` and `orbital continue` commands.

Verification: `make check` passes (lint + tests).

### REL-1: Propagate errors from Queue.Pop()

**Completed**

Implementation details:
1. Changed `Pop()` signature from `func (q *Queue) Pop() []string` to `func (q *Queue) Pop() ([]string, error)`
2. Wrapped the pop logic in `withLock()` for consistency with `Add()` and `Remove()` operations
3. Return error from `save()` instead of ignoring it
4. Updated callers in `cmd/orbital/root.go` and `cmd/orbital/continue.go` to handle the error
5. Updated existing tests to handle the new return signature
6. Added new test `TestQueue_Pop_ReturnsErrorWhenSaveFails` to verify error propagation

The fix ensures that if the queue file cannot be saved (disk full, permissions), the error is propagated to callers. The files are still returned to allow the caller to decide how to handle the situation (proceed with warning or fail).
