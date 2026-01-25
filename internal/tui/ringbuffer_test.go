package tui

import (
	"testing"

	"github.com/flashingpumpkin/orbital/internal/util"
)

func TestRingBuffer_NewRingBuffer(t *testing.T) {
	tests := []struct {
		name        string
		capacity    int
		wantCap     int
	}{
		{"normal capacity", 100, 100},
		{"zero capacity defaults", 0, DefaultMaxOutputLines},
		{"negative capacity defaults", -1, DefaultMaxOutputLines},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := NewRingBuffer(tt.capacity)
			if rb.Cap() != tt.wantCap {
				t.Errorf("Cap() = %d, want %d", rb.Cap(), tt.wantCap)
			}
			if rb.Len() != 0 {
				t.Errorf("Len() = %d, want 0 for new buffer", rb.Len())
			}
		})
	}
}

func TestRingBuffer_Push_BelowCapacity(t *testing.T) {
	rb := NewRingBuffer(5)

	rb.Push("a")
	rb.Push("b")
	rb.Push("c")

	if rb.Len() != 3 {
		t.Errorf("Len() = %d, want 3", rb.Len())
	}

	// Items should be in order
	if rb.Get(0) != "a" {
		t.Errorf("Get(0) = %q, want %q", rb.Get(0), "a")
	}
	if rb.Get(1) != "b" {
		t.Errorf("Get(1) = %q, want %q", rb.Get(1), "b")
	}
	if rb.Get(2) != "c" {
		t.Errorf("Get(2) = %q, want %q", rb.Get(2), "c")
	}
}

func TestRingBuffer_Push_AtCapacity(t *testing.T) {
	rb := NewRingBuffer(3)

	rb.Push("a")
	rb.Push("b")
	rb.Push("c")

	if rb.Len() != 3 {
		t.Errorf("Len() = %d, want 3", rb.Len())
	}

	// Push one more, should evict "a"
	rb.Push("d")

	if rb.Len() != 3 {
		t.Errorf("Len() = %d, want 3 after eviction", rb.Len())
	}

	// Now oldest is "b", newest is "d"
	if rb.Get(0) != "b" {
		t.Errorf("Get(0) = %q, want %q (oldest)", rb.Get(0), "b")
	}
	if rb.Get(1) != "c" {
		t.Errorf("Get(1) = %q, want %q", rb.Get(1), "c")
	}
	if rb.Get(2) != "d" {
		t.Errorf("Get(2) = %q, want %q (newest)", rb.Get(2), "d")
	}
}

func TestRingBuffer_Push_OverflowMultiple(t *testing.T) {
	rb := NewRingBuffer(3)

	// Push 6 items, should only keep last 3
	for i := 0; i < 6; i++ {
		rb.Push(util.IntToString(i))
	}

	if rb.Len() != 3 {
		t.Errorf("Len() = %d, want 3", rb.Len())
	}

	// Should contain "3", "4", "5"
	if rb.Get(0) != "3" {
		t.Errorf("Get(0) = %q, want %q", rb.Get(0), "3")
	}
	if rb.Get(1) != "4" {
		t.Errorf("Get(1) = %q, want %q", rb.Get(1), "4")
	}
	if rb.Get(2) != "5" {
		t.Errorf("Get(2) = %q, want %q", rb.Get(2), "5")
	}
}

func TestRingBuffer_Get_OutOfRange(t *testing.T) {
	rb := NewRingBuffer(5)
	rb.Push("a")
	rb.Push("b")

	tests := []struct {
		index int
		want  string
	}{
		{-1, ""},      // Negative index
		{2, ""},       // Past end
		{100, ""},     // Way past end
	}

	for _, tt := range tests {
		got := rb.Get(tt.index)
		if got != tt.want {
			t.Errorf("Get(%d) = %q, want %q", tt.index, got, tt.want)
		}
	}
}

func TestRingBuffer_ToSlice(t *testing.T) {
	t.Run("empty buffer", func(t *testing.T) {
		rb := NewRingBuffer(5)
		slice := rb.ToSlice()
		if len(slice) != 0 {
			t.Errorf("ToSlice() len = %d, want 0", len(slice))
		}
		if slice == nil {
			t.Error("ToSlice() should return empty slice, not nil")
		}
	})

	t.Run("partial buffer", func(t *testing.T) {
		rb := NewRingBuffer(5)
		rb.Push("a")
		rb.Push("b")

		slice := rb.ToSlice()
		if len(slice) != 2 {
			t.Errorf("ToSlice() len = %d, want 2", len(slice))
		}
		if slice[0] != "a" || slice[1] != "b" {
			t.Errorf("ToSlice() = %v, want [a, b]", slice)
		}
	})

	t.Run("wrapped buffer", func(t *testing.T) {
		rb := NewRingBuffer(3)
		rb.Push("a")
		rb.Push("b")
		rb.Push("c")
		rb.Push("d") // Evicts "a"

		slice := rb.ToSlice()
		if len(slice) != 3 {
			t.Errorf("ToSlice() len = %d, want 3", len(slice))
		}
		expected := []string{"b", "c", "d"}
		for i, want := range expected {
			if slice[i] != want {
				t.Errorf("ToSlice()[%d] = %q, want %q", i, slice[i], want)
			}
		}
	})
}

func TestRingBuffer_Clear(t *testing.T) {
	rb := NewRingBuffer(5)
	rb.Push("a")
	rb.Push("b")
	rb.Push("c")

	rb.Clear()

	if rb.Len() != 0 {
		t.Errorf("Len() = %d after Clear(), want 0", rb.Len())
	}

	// Should be able to push again after clear
	rb.Push("x")
	if rb.Len() != 1 {
		t.Errorf("Len() = %d after push, want 1", rb.Len())
	}
	if rb.Get(0) != "x" {
		t.Errorf("Get(0) = %q, want %q", rb.Get(0), "x")
	}
}

func TestRingBuffer_Iterate(t *testing.T) {
	t.Run("full iteration", func(t *testing.T) {
		rb := NewRingBuffer(5)
		rb.Push("a")
		rb.Push("b")
		rb.Push("c")

		var items []string
		rb.Iterate(func(_ int, item string) bool {
			items = append(items, item)
			return true
		})

		if len(items) != 3 {
			t.Errorf("Iterate() collected %d items, want 3", len(items))
		}
		expected := []string{"a", "b", "c"}
		for i, want := range expected {
			if items[i] != want {
				t.Errorf("items[%d] = %q, want %q", i, items[i], want)
			}
		}
	})

	t.Run("early termination", func(t *testing.T) {
		rb := NewRingBuffer(5)
		rb.Push("a")
		rb.Push("b")
		rb.Push("c")

		count := 0
		rb.Iterate(func(_ int, _ string) bool {
			count++
			return count < 2 // Stop after 2 items
		})

		if count != 2 {
			t.Errorf("Iterate() called %d times, want 2", count)
		}
	})

	t.Run("wrapped buffer iteration", func(t *testing.T) {
		rb := NewRingBuffer(3)
		rb.Push("1")
		rb.Push("2")
		rb.Push("3")
		rb.Push("4") // Evicts "1"

		var items []string
		rb.Iterate(func(_ int, item string) bool {
			items = append(items, item)
			return true
		})

		expected := []string{"2", "3", "4"}
		if len(items) != len(expected) {
			t.Errorf("Iterate() collected %d items, want %d", len(items), len(expected))
		}
		for i, want := range expected {
			if items[i] != want {
				t.Errorf("items[%d] = %q, want %q", i, items[i], want)
			}
		}
	})
}

func TestRingBuffer_MemoryBound(t *testing.T) {
	// Test that buffer maintains exactly 10000 lines after many pushes
	rb := NewRingBuffer(10000)

	// Push 50000 lines
	for i := 0; i < 50000; i++ {
		rb.Push("line " + util.IntToString(i))
	}

	if rb.Len() != 10000 {
		t.Errorf("Len() = %d after 50000 pushes, want exactly 10000", rb.Len())
	}

	// Oldest should be line 40000 (50000 - 10000)
	oldest := rb.Get(0)
	if oldest != "line 40000" {
		t.Errorf("oldest line = %q, want %q", oldest, "line 40000")
	}

	// Newest should be line 49999
	newest := rb.Get(9999)
	if newest != "line 49999" {
		t.Errorf("newest line = %q, want %q", newest, "line 49999")
	}
}
