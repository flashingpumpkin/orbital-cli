package tui

import "testing"

func TestCalculateLayout(t *testing.T) {
	tests := []struct {
		name       string
		width      int
		height     int
		taskCount  int
		wantTooSmall bool
		wantScrollHeight int
		wantTaskHeight   int
	}{
		{
			name:             "standard terminal no tasks",
			width:            120,
			height:           40,
			taskCount:        0,
			wantTooSmall:     false,
			wantScrollHeight: 32, // 40 - (0 + 2 + 2 + 4)
			wantTaskHeight:   0,
		},
		{
			name:             "standard terminal with 3 tasks",
			width:            120,
			height:           40,
			taskCount:        3,
			wantTooSmall:     false,
			wantScrollHeight: 28, // 40 - (4 + 2 + 2 + 4)
			wantTaskHeight:   4,  // 3 tasks + 1 header
		},
		{
			name:             "standard terminal with max tasks",
			width:            120,
			height:           40,
			taskCount:        6,
			wantTooSmall:     false,
			wantScrollHeight: 25, // 40 - (7 + 2 + 2 + 4)
			wantTaskHeight:   7,  // 6 tasks + 1 header
		},
		{
			name:             "standard terminal with overflow tasks",
			width:            120,
			height:           40,
			taskCount:        10,
			wantTooSmall:     false,
			wantScrollHeight: 25, // 40 - (7 + 2 + 2 + 4) - capped at max
			wantTaskHeight:   7,  // max 6 + 1 header
		},
		{
			name:           "too narrow",
			width:          60,
			height:         40,
			taskCount:      0,
			wantTooSmall:   true,
		},
		{
			name:           "too short",
			width:          120,
			height:         20,
			taskCount:      0,
			wantTooSmall:   true,
		},
		{
			name:             "minimum viable size no tasks",
			width:            80,
			height:           24,
			taskCount:        0,
			wantTooSmall:     false,
			wantScrollHeight: 16, // 24 - (0 + 2 + 2 + 4)
			wantTaskHeight:   0,
		},
		{
			name:             "minimum viable size with tasks",
			width:            80,
			height:           24,
			taskCount:        6,
			wantTooSmall:     false,
			wantScrollHeight: 9, // 24 - (7 + 2 + 2 + 4)
			wantTaskHeight:   7, // 6 tasks + 1 header
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := CalculateLayout(tt.width, tt.height, tt.taskCount)

			if layout.TooSmall != tt.wantTooSmall {
				t.Errorf("TooSmall = %v, want %v", layout.TooSmall, tt.wantTooSmall)
			}

			if tt.wantTooSmall {
				if layout.TooSmallMessage == "" {
					t.Error("TooSmallMessage should not be empty when TooSmall is true")
				}
				return
			}

			if layout.ScrollAreaHeight != tt.wantScrollHeight {
				t.Errorf("ScrollAreaHeight = %d, want %d", layout.ScrollAreaHeight, tt.wantScrollHeight)
			}

			if layout.TaskPanelHeight != tt.wantTaskHeight {
				t.Errorf("TaskPanelHeight = %d, want %d", layout.TaskPanelHeight, tt.wantTaskHeight)
			}
		})
	}
}

func TestLayoutContentWidth(t *testing.T) {
	layout := CalculateLayout(100, 40, 0)
	if layout.ContentWidth() != 98 {
		t.Errorf("ContentWidth() = %d, want 98", layout.ContentWidth())
	}
}

func TestLayoutTasksVisible(t *testing.T) {
	tests := []struct {
		name        string
		taskCount   int
		wantVisible int
	}{
		{"no tasks", 0, 0},
		{"3 tasks", 3, 3},
		{"6 tasks", 6, 6},
		{"10 tasks capped", 10, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := CalculateLayout(120, 40, tt.taskCount)
			visible := layout.TasksVisible()
			if visible != tt.wantVisible {
				t.Errorf("TasksVisible() = %d, want %d", visible, tt.wantVisible)
			}
		})
	}
}

func TestLayoutHasTaskOverflow(t *testing.T) {
	tests := []struct {
		name         string
		taskCount    int
		wantOverflow bool
	}{
		{"no overflow with 3 tasks", 3, false},
		{"no overflow with 6 tasks", 6, false},
		{"overflow with 7 tasks", 7, true},
		{"overflow with 10 tasks", 10, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layout := CalculateLayout(120, 40, tt.taskCount)
			overflow := layout.HasTaskOverflow(tt.taskCount)
			if overflow != tt.wantOverflow {
				t.Errorf("HasTaskOverflow(%d) = %v, want %v", tt.taskCount, overflow, tt.wantOverflow)
			}
		})
	}
}
