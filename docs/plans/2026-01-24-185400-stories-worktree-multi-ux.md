# User Stories: Multi-Worktree UX Improvements

## Problem Statement

Orbital currently supports multiple worktrees but the UX is poor:

1. **No list command**: Can't see what worktrees exist
2. **No selection on resume**: First worktree is always used
3. **No status visibility**: Can't see worktree health without reading JSON

**Affected code:**
- `cmd/orbital/continue.go:66-69` - First worktree selection
- No existing list/status command

---

## User Story 1: Add Worktree List Command

**As a** developer
**I want** to see the status of all worktrees
**So that** I can understand what's active and identify issues

### Context

There's no way to inspect worktree state without reading JSON manually.

### Acceptance Criteria

- [ ] Given worktrees exist, when `orbital worktree list` runs, then all worktrees are shown
- [ ] Given output is displayed, when user reads it, then each worktree shows: name, path, branch, original branch, created date, spec files
- [ ] Given a worktree path doesn't exist on disk, when listed, then it's marked as "MISSING"
- [ ] Given a worktree is not registered with git, when listed, then it's marked as "ORPHAN"
- [ ] Given `--json` flag is set, when list runs, then output is JSON for scripting
- [ ] Given no worktrees exist, when list runs, then message says "No active worktrees"

### Technical Notes

```go
// cmd/orbital/worktree.go

var worktreeCmd = &cobra.Command{
    Use:   "worktree",
    Short: "Manage git worktrees",
}

var listCmd = &cobra.Command{
    Use:   "list",
    Short: "List all tracked worktrees",
    RunE:  runWorktreeList,
}

func init() {
    rootCmd.AddCommand(worktreeCmd)
    worktreeCmd.AddCommand(listCmd)
    listCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
}

type WorktreeListItem struct {
    Name           string   `json:"name"`
    Path           string   `json:"path"`
    Branch         string   `json:"branch"`
    OriginalBranch string   `json:"original_branch"`
    SpecFiles      []string `json:"spec_files"`
    CreatedAt      string   `json:"created_at"`
    Status         string   `json:"status"` // "OK", "MISSING", "ORPHAN"
}

func runWorktreeList(cmd *cobra.Command, args []string) error {
    sm := worktree.NewStateManager(workingDir)
    state, err := sm.Load()
    if err != nil {
        return err
    }

    if len(state.Worktrees) == 0 {
        fmt.Println("No active worktrees")
        return nil
    }

    // Check status of each worktree
    items := make([]WorktreeListItem, len(state.Worktrees))
    for i, wt := range state.Worktrees {
        status := checkWorktreeStatus(wt.Path)
        items[i] = WorktreeListItem{
            Name:           wt.Name,
            Path:           wt.Path,
            Branch:         wt.Branch,
            OriginalBranch: wt.OriginalBranch,
            SpecFiles:      wt.SpecFiles,
            CreatedAt:      wt.CreatedAt.Format(time.RFC3339),
            Status:         status,
        }
    }

    if jsonOutput {
        return json.NewEncoder(os.Stdout).Encode(items)
    }

    // Pretty print table
    printWorktreeTable(items)
    return nil
}
```

### Definition of Done

- [ ] `orbital worktree list` command implemented
- [ ] Shows all worktrees with status
- [ ] Status indicates MISSING/ORPHAN paths
- [ ] `--json` flag for machine-readable output
- [ ] Tests for list command
- [ ] All existing tests pass

**Effort Estimate**: S

---

## User Story 2: Add Worktree Selection on Resume

**As a** developer with multiple active worktrees
**I want** to choose which worktree to resume
**So that** I can work on multiple features in parallel

### Context

`continue.go:66-69` only uses the first worktree:

```go
if err == nil && len(worktrees) > 0 {
    wt := worktrees[0]  // Always first!
    wtState = &wt
}
```

### Acceptance Criteria

- [ ] Given multiple worktrees exist, when `orbital continue` runs, then user is prompted to select one
- [ ] Given `--worktree-name` flag is set, when continue runs, then specified worktree is used
- [ ] Given only one worktree exists, when continue runs, then it's used automatically
- [ ] Given selection is displayed, when user chooses, then format shows name, branch, and spec files
- [ ] Given invalid selection, when user enters it, then prompt repeats with error message
- [ ] Given `--non-interactive` flag is set, when multiple exist, then error is returned

### Technical Notes

```go
// cmd/orbital/continue.go

func selectWorktree(worktrees []worktree.WorktreeState) (*worktree.WorktreeState, error) {
    if len(worktrees) == 0 {
        return nil, nil
    }
    if len(worktrees) == 1 {
        return &worktrees[0], nil
    }

    // Check for --worktree-name flag
    if worktreeName != "" {
        for i := range worktrees {
            if worktrees[i].Name == worktreeName {
                return &worktrees[i], nil
            }
        }
        return nil, fmt.Errorf("worktree %q not found", worktreeName)
    }

    // Interactive selection
    if nonInteractive {
        return nil, fmt.Errorf("multiple worktrees found, use --worktree-name to select:\n%s",
            formatWorktreeList(worktrees))
    }

    fmt.Println("Multiple worktrees found. Select one to resume:")
    for i, wt := range worktrees {
        fmt.Printf("  [%d] %s (branch: %s)\n", i+1, wt.Name, wt.Branch)
        fmt.Printf("      Spec: %s\n", strings.Join(wt.SpecFiles, ", "))
    }

    fmt.Print("\nEnter number: ")
    var selection int
    if _, err := fmt.Scanf("%d", &selection); err != nil {
        return nil, fmt.Errorf("invalid selection: %w", err)
    }

    if selection < 1 || selection > len(worktrees) {
        return nil, fmt.Errorf("selection %d out of range (1-%d)", selection, len(worktrees))
    }

    return &worktrees[selection-1], nil
}
```

### Definition of Done

- [ ] Interactive selection when multiple worktrees exist
- [ ] `--worktree-name` flag for direct selection
- [ ] `--non-interactive` flag for scripts
- [ ] Clear display of options
- [ ] Tests for selection logic
- [ ] All existing tests pass

**Effort Estimate**: S

---

## User Story 3: Add Worktree Show Command

**As a** developer
**I want** to see detailed status of a specific worktree
**So that** I can diagnose issues and understand its state

### Context

The list command shows summary info, but sometimes more detail is needed for debugging.

### Acceptance Criteria

- [ ] Given worktree name is provided, when `orbital worktree show <name>` runs, then detailed info is displayed
- [ ] Given output is displayed, when user reads it, then it includes: git status, last commit, divergence from original
- [ ] Given worktree has uncommitted changes, when shown, then changes are listed
- [ ] Given worktree branch has diverged from original, when shown, then commit difference is shown
- [ ] Given worktree doesn't exist, when show runs, then error says "worktree not found"

### Technical Notes

```go
// cmd/orbital/worktree.go

var showCmd = &cobra.Command{
    Use:   "show <name>",
    Short: "Show detailed status of a worktree",
    Args:  cobra.ExactArgs(1),
    RunE:  runWorktreeShow,
}

func runWorktreeShow(cmd *cobra.Command, args []string) error {
    name := args[0]

    sm := worktree.NewStateManager(workingDir)
    state, err := sm.Load()
    if err != nil {
        return err
    }

    // Find worktree by name
    var wt *worktree.WorktreeState
    for i := range state.Worktrees {
        if state.Worktrees[i].Name == name {
            wt = &state.Worktrees[i]
            break
        }
    }
    if wt == nil {
        return fmt.Errorf("worktree %q not found", name)
    }

    // Display basic info
    fmt.Printf("Name:            %s\n", wt.Name)
    fmt.Printf("Path:            %s\n", wt.Path)
    fmt.Printf("Branch:          %s\n", wt.Branch)
    fmt.Printf("Original Branch: %s\n", wt.OriginalBranch)
    fmt.Printf("Created:         %s\n", wt.CreatedAt.Format(time.RFC3339))
    fmt.Printf("Spec Files:      %s\n", strings.Join(wt.SpecFiles, ", "))
    fmt.Println()

    // Check if path exists
    if _, err := os.Stat(wt.Path); os.IsNotExist(err) {
        fmt.Println("Status: MISSING (path does not exist)")
        return nil
    }

    // Get git status
    statusCmd := exec.Command("git", "-C", wt.Path, "status", "--short")
    statusOutput, _ := statusCmd.Output()
    if len(statusOutput) > 0 {
        fmt.Println("Uncommitted Changes:")
        fmt.Println(string(statusOutput))
    } else {
        fmt.Println("Working tree clean")
    }

    // Get last commit
    logCmd := exec.Command("git", "-C", wt.Path, "log", "-1", "--oneline")
    logOutput, _ := logCmd.Output()
    fmt.Printf("Last Commit: %s\n", strings.TrimSpace(string(logOutput)))

    // Check divergence
    aheadCmd := exec.Command("git", "-C", wt.Path, "rev-list", "--count",
        fmt.Sprintf("%s..%s", wt.OriginalBranch, wt.Branch))
    aheadOutput, _ := aheadCmd.Output()
    ahead, _ := strconv.Atoi(strings.TrimSpace(string(aheadOutput)))

    behindCmd := exec.Command("git", "-C", wt.Path, "rev-list", "--count",
        fmt.Sprintf("%s..%s", wt.Branch, wt.OriginalBranch))
    behindOutput, _ := behindCmd.Output()
    behind, _ := strconv.Atoi(strings.TrimSpace(string(behindOutput)))

    fmt.Printf("Divergence:  %d ahead, %d behind %s\n", ahead, behind, wt.OriginalBranch)

    return nil
}
```

### Definition of Done

- [ ] `orbital worktree show <name>` command implemented
- [ ] Shows basic info from state
- [ ] Shows git status (uncommitted changes)
- [ ] Shows last commit
- [ ] Shows divergence from original branch
- [ ] Tests for show command
- [ ] All existing tests pass

**Effort Estimate**: S

---

## User Story 4: Add Worktree Remove Command

**As a** developer
**I want** to manually remove a specific worktree
**So that** I can clean up without completing the work

### Context

Currently worktrees can only be removed via the cleanup command (which removes all orphans) or by completing the work (merge + cleanup). There's no way to abandon a specific worktree.

### Acceptance Criteria

- [ ] Given worktree name is provided, when `orbital worktree remove <name>` runs, then worktree is removed
- [ ] Given worktree has uncommitted changes, when remove runs, then confirmation is required
- [ ] Given `--force` flag is set, when remove runs, then no confirmation is asked
- [ ] Given worktree doesn't exist, when remove runs, then error says "worktree not found"
- [ ] Given removal succeeds, when complete, then state file is updated

### Technical Notes

```go
// cmd/orbital/worktree.go

var removeCmd = &cobra.Command{
    Use:   "remove <name>",
    Short: "Remove a worktree and its branch",
    Args:  cobra.ExactArgs(1),
    RunE:  runWorktreeRemove,
}

func runWorktreeRemove(cmd *cobra.Command, args []string) error {
    name := args[0]

    sm := worktree.NewStateManager(workingDir)
    state, err := sm.Load()
    if err != nil {
        return err
    }

    // Find worktree
    var wt *worktree.WorktreeState
    for i := range state.Worktrees {
        if state.Worktrees[i].Name == name {
            wt = &state.Worktrees[i]
            break
        }
    }
    if wt == nil {
        return fmt.Errorf("worktree %q not found", name)
    }

    // Check for uncommitted changes
    if !forceFlag {
        // ... check and prompt
    }

    // Remove worktree and branch
    cleanup := worktree.NewCleanup(workingDir)
    if err := cleanup.Run(wt.Path, wt.Branch); err != nil {
        return fmt.Errorf("failed to remove worktree: %w", err)
    }

    // Update state
    if err := sm.Remove(wt.Path); err != nil {
        return fmt.Errorf("failed to update state: %w", err)
    }

    fmt.Printf("Removed worktree %q\n", name)
    return nil
}
```

### Definition of Done

- [ ] `orbital worktree remove <name>` command implemented
- [ ] Checks for uncommitted changes
- [ ] `--force` flag skips confirmation
- [ ] Updates state file after removal
- [ ] Tests for remove command
- [ ] All existing tests pass

**Effort Estimate**: S

---

## Implementation Order

1. **Story 1** (List command) - Foundation for visibility
2. **Story 4** (Remove command) - Enables manual cleanup
3. **Story 2** (Selection on resume) - Better multi-worktree UX
4. **Story 3** (Show command) - Advanced debugging

## Verification

```bash
# Create multiple worktrees
orbital --worktree spec1.md
orbital --worktree spec2.md

# List worktrees
orbital worktree list
orbital worktree list --json

# Show specific worktree
orbital worktree show swift-falcon

# Resume with selection
orbital continue  # Should prompt for selection
orbital continue --worktree-name swift-falcon

# Remove specific worktree
orbital worktree remove calm-otter
orbital worktree remove swift-falcon --force
```

---

## Dependencies

- All stories depend on state management
- Stories are independent of each other

## Risks

- Interactive selection doesn't work in CI/scripts (mitigated by `--non-interactive`)
- Remove command is destructive (mitigated by uncommitted change check)
