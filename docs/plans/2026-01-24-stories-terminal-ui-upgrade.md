# Terminal UI Upgrade

## Overview

Replace the current scrolling output with a structured terminal UI featuring a fixed status panel at the bottom and scrolling output above. Key metrics, workflow state, and task progress remain visible at all times while Claude's output scrolls past.

## Technology Decision

Use **bubbletea** with **lipgloss** for the terminal UI implementation.

- `github.com/charmbracelet/bubbletea` - Elm-architecture TUI framework
- `github.com/charmbracelet/lipgloss` - Declarative styling for terminal output

Rationale:
- Clean separation between model, update, and view
- Built-in resize handling
- Straightforward composition of UI components
- Active maintenance and good documentation
- lipgloss provides CSS-like styling without manual ANSI codes

## Layout

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│  [Scrolling Claude Output]                                      │
│                                                                 │
│  > Implementing user authentication...                          │
│  > Created src/auth/login.go                                    │
│  > Running tests...                                             │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│  Tasks                                                          │
│  ✓ Set up auth middleware                                       │
│  → Implement login endpoint                                     │
│  ○ Add session management                                       │
│  ○ Write integration tests                                      │
├─────────────────────────────────────────────────────────────────┤
│  Iteration 3/50 │ Step: implement (2/4) │ Gate retries: 0/3     │
│  Tokens: 45,231 in / 12,847 out │ Cost: $2.34 / $10.00          │
├─────────────────────────────────────────────────────────────────┤
│  Spec: docs/plans/auth-feature.md                               │
│  Notes: .orbit/notes.md │ State: .orbit/state.json              │
└─────────────────────────────────────────────────────────────────┘
```

## Panel Sections

### Scrolling Output (top)
- Claude's streamed text output
- Tool invocations and results (summarised)
- Scrolls upward as new content arrives
- Takes remaining vertical space

### Tasks Panel (fixed)
- Shows Claude's current todo/task list
- Updates in real-time as Claude modifies tasks
- Icons: `✓` complete, `→` in progress, `○` pending
- Collapses if no tasks exist
- Max height: 6 lines (scrollable if more tasks)

### Progress Bar (fixed)
- Current iteration number and max
- Current workflow step name and position
- Gate retry count (if in gated workflow)
- Token counts (input/output)
- Running cost and budget limit

### Session Info (fixed)
- Spec file path(s)
- Notes file path
- State file path
- Context file path (if applicable)

## User Stories

---

### Story 1: Fixed Status Bar Layout

**As a** user
**I want** a fixed status bar at the bottom of the terminal
**So that** I can always see progress without scrolling

**Acceptance Criteria**

- [x] Terminal splits into scrolling area (top) and fixed panels (bottom)
- [x] Fixed panels remain visible regardless of output length
- [x] Layout adapts to terminal width
- [x] Minimum terminal width enforced (80 chars) with graceful message if smaller
- [x] Terminal resize is handled correctly

**Definition of Done**

- [x] Layout rendering with terminal size detection
- [x] Resize handler updates layout
- [x] Manual testing at various terminal sizes

---

### Story 2: Real-time Token and Cost Display

**As a** user
**I want** to see tokens and cost update continuously
**So that** I can monitor spend as Claude works

**Acceptance Criteria**

- [ ] Input tokens displayed and updated after each Claude response
- [ ] Output tokens displayed and updated after each Claude response
- [ ] Running cost calculated and displayed (cumulative across iteration)
- [ ] Budget limit shown alongside current cost
- [ ] Cost formatted as currency ($X.XX)
- [ ] Tokens formatted with thousands separator (45,231)
- [ ] Warning colour when cost exceeds 80% of budget

**Definition of Done**

- [ ] Token aggregation from stream-json parser
- [ ] Cost calculation logic
- [ ] Unit tests for formatting
- [ ] Visual indicator at budget thresholds

---

### Story 3: Workflow Step Progress Display

**As a** user
**I want** to see the current workflow step
**So that** I know which phase of the iteration is running

**Acceptance Criteria**

- [ ] Current step name displayed (e.g., "implement", "review", "refactor")
- [ ] Step position shown (e.g., "2/4" for second of four steps)
- [ ] Step name updates when workflow advances
- [ ] Gate retry count shown for gated steps
- [ ] Visual distinction when on a gate step

**Definition of Done**

- [ ] Step tracking in controller
- [ ] Display updates on step transition
- [ ] Unit tests for step progression display

---

### Story 4: Iteration Counter Display

**As a** user
**I want** to see the current iteration number
**So that** I know how many loops have run

**Acceptance Criteria**

- [ ] Current iteration number displayed
- [ ] Maximum iterations shown (e.g., "3/50")
- [ ] Updates at start of each new iteration
- [ ] Warning colour when approaching max iterations (>80%)

**Definition of Done**

- [ ] Iteration count passed to display
- [ ] Threshold colour logic
- [ ] Unit tests for iteration display

---

### Story 5: Task List Panel

**As a** user
**I want** to see Claude's tasks/todos as they're created and updated
**So that** I can track progress on sub-tasks

**Acceptance Criteria**

- [ ] Parse task creation from Claude's output (TaskCreate tool use)
- [ ] Parse task updates from Claude's output (TaskUpdate tool use)
- [ ] Display task list with status icons: `✓` complete, `→` in progress, `○` pending
- [ ] Tasks update in real-time as Claude modifies them
- [ ] Panel collapses/hides when no tasks exist
- [ ] Maximum 6 visible tasks; scroll indicator if more
- [ ] Task subject truncated with ellipsis if too long for width

**Definition of Done**

- [ ] Task parsing from stream-json
- [ ] Task state management
- [ ] Panel rendering with overflow handling
- [ ] Unit tests for task state transitions

---

### Story 6: Session Info Display

**As a** user
**I want** to see session file paths at all times
**So that** I know which files Orbit is working with

**Acceptance Criteria**

- [ ] Spec file path(s) displayed
- [ ] Notes file path displayed
- [ ] State file path displayed
- [ ] Context file path displayed (if configured)
- [ ] Paths truncated from left with "..." if too long for terminal width
- [ ] Multiple spec files shown comma-separated or with count

**Definition of Done**

- [ ] Path display with truncation logic
- [ ] Multi-file handling
- [ ] Unit tests for path formatting

---

### Story 7: Scrolling Output Area

**As a** user
**I want** Claude's output to scroll above the fixed panels
**So that** I can see recent output while status remains visible

**Acceptance Criteria**

- [ ] Output text streams into scrolling area
- [ ] New output pushes older content up
- [ ] Scrolling area respects fixed panel heights
- [ ] Colour formatting preserved in output
- [ ] Long lines wrap correctly
- [ ] Output can be scrolled back (optional: with keyboard)

**Definition of Done**

- [ ] Scroll region implementation
- [ ] ANSI colour passthrough
- [ ] Line wrapping logic
- [ ] Integration test with long output

---

### Story 8: Colour and Visual Hierarchy

**As a** user
**I want** clear visual distinction between UI elements
**So that** I can quickly parse the display

**Acceptance Criteria**

- [ ] Panel borders in subtle colour (dim/grey)
- [ ] Labels in one colour, values in another
- [ ] Warning states in yellow/orange
- [ ] Error states in red
- [ ] Success states in green
- [ ] Current/active items highlighted
- [ ] Respects NO_COLOR environment variable

**Definition of Done**

- [ ] Colour scheme defined
- [ ] NO_COLOR detection
- [ ] Consistent application across all panels

---

## Technical Notes

### Stream-JSON Task Parsing

Tasks appear in Claude's output as tool uses:

```json
{"type": "tool_use", "name": "TaskCreate", "input": {"subject": "...", "description": "..."}}
{"type": "tool_result", "content": {"id": "task-1", ...}}
{"type": "tool_use", "name": "TaskUpdate", "input": {"taskId": "task-1", "status": "completed"}}
```

Parse these from the stream to maintain task state.

### Layout Calculations

```
Total height = terminal rows
Fixed panels = tasks (variable, max 8) + progress (2) + session (2) + borders (4)
Scroll area = total height - fixed panels
```

Minimum viable: 24 rows. Below this, collapse task panel.

### Performance Considerations

- Buffer output updates; don't redraw on every character
- Batch task state updates
- Use terminal's scroll region for efficient scrolling
- Debounce resize events

## Alternative: Minimal Mode

For CI/non-interactive environments, provide `--minimal` flag:

- No fixed panels
- Simple streaming output
- Progress updates as periodic log lines
- Task updates as log lines

```
[iter 3/50] [step: implement 2/4] [cost: $2.34/$10.00]
> Implementing user authentication...
[task] ✓ Set up auth middleware
[task] → Implement login endpoint
```
