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
