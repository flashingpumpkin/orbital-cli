# User Stories: TUI Redesign

**Date**: 2026-01-25
**Status**: Complete
**PRD**: `docs/plans/2026-01-25-140000-prd-tui-redesign.md`

## Overview

Implementation tracking for the Amber Terminal TUI redesign as specified in the PRD.

## Phase 1: Style System Refactor

### [x] **Story 1.1: Create centralised style constants**

**As a** developer maintaining the TUI
**I want** all colours and styles defined in one place
**So that** the visual identity is consistent and easy to update

**Acceptance Criteria**:
- [x] Create `internal/tui/styles.go` with colour palette constants
- [x] Define box-drawing character constants
- [x] Define progress bar characters
- [x] Define status indicator icons
- [x] Create `Styles` struct with all lipgloss styles
- [x] Update `defaultStyles()` to use amber palette

---

## Phase 2: Layout Enhancement

### [x] **Story 2.1: Add header panel to layout**

**As a** user watching Orbital run
**I want** to see key metrics at the top
**So that** I can monitor progress at a glance

**Acceptance Criteria**:
- [x] Add `HeaderPanelHeight` constant (1 line)
- [x] Add `HelpBarHeight` constant (1 line)
- [x] Update `Layout` struct with new fields
- [x] Update `CalculateLayout()` to include header and help bar
- [x] Update tests for new layout calculations

---

## Phase 3: Visual Updates

### [x] **Story 3.1: Implement header panel**

**As a** user watching Orbital run
**I want** to see brand and metrics in the header
**So that** I can identify the tool and see status

**Acceptance Criteria**:
- [x] Implement `renderHeader()` method
- [x] Display brand mark (◆ ORBITAL)
- [x] Display iteration count with warning colour when >80%
- [x] Display cost/budget with warning colour when >80%

### [x] **Story 3.2: Implement help bar**

**As a** user interacting with the TUI
**I want** to see keyboard shortcuts
**So that** I know how to navigate

**Acceptance Criteria**:
- [x] Implement `renderHelpBar()` method
- [x] Display shortcuts outside main frame
- [x] Use dim amber styling

### [x] **Story 3.3: Update tab bar styling**

**As a** user switching between tabs
**I want** clear visual distinction
**So that** I know which tab is active

**Acceptance Criteria**:
- [x] Active tab uses amber background
- [x] Inactive tabs use faded amber
- [x] Add border characters to tab bar

### [x] **Story 3.4: Add progress bar visualisations**

**As a** user monitoring progress
**I want** visual progress bars
**So that** I can quickly see how much is used

**Acceptance Criteria**:
- [x] Implement `RenderProgressBar()` function
- [x] Add iteration progress bar to progress panel
- [x] Add budget progress bar to progress panel
- [x] Bars change colour at >80% threshold

### [x] **Story 3.5: Update task panel icons**

**As a** user viewing tasks
**I want** clear status indicators
**So that** I can see task states at a glance

**Acceptance Criteria**:
- [x] Use ● for completed (green)
- [x] Use → for in progress (amber)
- [x] Use ○ for pending (dim)

### [x] **Story 3.6: Update all panels with borders**

**As a** user viewing the TUI
**I want** a framed appearance
**So that** it looks professional

**Acceptance Criteria**:
- [x] Add double-line borders to outer frame
- [x] Update renderScrollArea() with borders
- [x] Update renderFileContent() with borders
- [x] Update renderTaskPanel() with borders
- [x] Update renderProgressPanel() with borders
- [x] Update renderSessionPanel() with borders
- [x] Update renderWorktreePanel() with borders

---

## Phase 4: Session Selector

### [x] **Story 4.1: Update selector with amber styling**

**As a** user selecting a session
**I want** consistent visual style
**So that** the experience is cohesive

**Acceptance Criteria**:
- [x] Update `selector/styles.go` with amber palette
- [x] Add border helper functions
- [x] Update `viewSessionList()` with bordered frame
- [x] Update `viewCleanupDialog()` with bordered frame
- [x] Update help bar styling

---

## Phase 5: Polish

### [x] **Story 5.1: Add loading spinners** (SKIPPED - Optional)

**As a** user waiting for operations
**I want** animated indicators
**So that** I know the system is working

**Status**: Skipped (marked as optional in PRD)

**Acceptance Criteria**:
- [x] N/A - PRD marks spinners as "(Optional)"

### [x] **Story 5.2: Improve empty state messages**

**As a** user with no output
**I want** helpful empty states
**So that** I understand the current state

**Acceptance Criteria**:
- [x] Add styled empty state for output tab (centred "Waiting for output..." message)
- [x] Add styled empty state for no tasks (task panel hidden when no tasks - clean design)

### [x] **Story 5.3: Test with various terminal sizes**

**As a** developer maintaining the TUI
**I want** confidence in different sizes
**So that** users have good experience everywhere

**Acceptance Criteria**:
- [x] Test with 80 column terminal (TestMinimumTerminalRendering)
- [x] Test with 200+ column terminal (TestWideTerminalRendering)
- [x] Verify NO_COLOR support still works (handled in program.go with lipgloss.SetColorProfile)

---

## Summary

| Phase | Stories | Status |
|-------|---------|--------|
| Phase 1: Style System | 1 | Complete |
| Phase 2: Layout Enhancement | 1 | Complete |
| Phase 3: Visual Updates | 6 | Complete |
| Phase 4: Session Selector | 1 | Complete |
| Phase 5: Polish | 3 | Complete |

**Overall Progress**: 12/12 stories complete
