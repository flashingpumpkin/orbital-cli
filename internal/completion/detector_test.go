package completion

import "testing"

func TestNew(t *testing.T) {
	t.Run("creates detector with promise", func(t *testing.T) {
		promise := "TASK_COMPLETE"
		d := New(promise)

		if d == nil {
			t.Fatal("expected detector to be created, got nil")
		}
		if d.promise != promise {
			t.Errorf("expected promise %q, got %q", promise, d.promise)
		}
	})

	t.Run("creates detector with empty promise", func(t *testing.T) {
		d := New("")
		if d == nil {
			t.Fatal("expected detector to be created, got nil")
		}
		if d.promise != "" {
			t.Errorf("expected empty promise, got %q", d.promise)
		}
	})
}

func TestCheck(t *testing.T) {
	t.Run("returns true when promise at start of output", func(t *testing.T) {
		d := New("DONE")
		output := "DONE: task completed successfully"

		if !d.Check(output) {
			t.Error("expected Check to return true when promise at start")
		}
	})

	t.Run("returns true when promise in middle of output", func(t *testing.T) {
		d := New("DONE")
		output := "Processing... DONE and finished"

		if !d.Check(output) {
			t.Error("expected Check to return true when promise in middle")
		}
	})

	t.Run("returns true when promise at end of output", func(t *testing.T) {
		d := New("DONE")
		output := "All tasks completed: DONE"

		if !d.Check(output) {
			t.Error("expected Check to return true when promise at end")
		}
	})

	t.Run("returns false when promise not in output", func(t *testing.T) {
		d := New("DONE")
		output := "Still processing..."

		if d.Check(output) {
			t.Error("expected Check to return false when promise not found")
		}
	})

	t.Run("is case sensitive", func(t *testing.T) {
		d := New("DONE")

		if d.Check("done") {
			t.Error("expected Check to be case-sensitive, 'done' should not match 'DONE'")
		}
		if d.Check("Done") {
			t.Error("expected Check to be case-sensitive, 'Done' should not match 'DONE'")
		}
	})

	t.Run("handles multiline output", func(t *testing.T) {
		d := New("COMPLETE")
		output := `Line 1: Starting process
Line 2: Processing data
Line 3: COMPLETE
Line 4: Cleanup done`

		if !d.Check(output) {
			t.Error("expected Check to return true for multiline output with promise")
		}
	})

	t.Run("handles multiline output without promise", func(t *testing.T) {
		d := New("COMPLETE")
		output := `Line 1: Starting process
Line 2: Processing data
Line 3: Still working
Line 4: Cleanup done`

		if d.Check(output) {
			t.Error("expected Check to return false for multiline output without promise")
		}
	})

	t.Run("returns false for empty output", func(t *testing.T) {
		d := New("DONE")

		if d.Check("") {
			t.Error("expected Check to return false for empty output")
		}
	})

	t.Run("returns true for empty promise in non-empty output", func(t *testing.T) {
		d := New("")

		if !d.Check("any output") {
			t.Error("expected Check to return true when promise is empty string")
		}
	})

	t.Run("handles exact match only", func(t *testing.T) {
		d := New("TASK")

		if !d.Check("TASK") {
			t.Error("expected Check to find exact match")
		}
	})
}

func TestExtractContext(t *testing.T) {
	t.Run("returns empty string when promise not found", func(t *testing.T) {
		d := New("MISSING")
		output := "This output does not contain the promise"

		result := d.ExtractContext(output)
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("returns context around promise in middle", func(t *testing.T) {
		d := New("FOUND")
		output := "prefix text FOUND suffix text"

		result := d.ExtractContext(output)
		if result == "" {
			t.Fatal("expected non-empty context")
		}
		if len(result) == 0 {
			t.Error("expected context to contain text")
		}
	})

	t.Run("limits context to 50 chars before promise", func(t *testing.T) {
		d := New("MARKER")
		// Create output with more than 50 chars before marker
		prefix := "0123456789012345678901234567890123456789012345678901234567890"
		output := prefix + "MARKER"

		result := d.ExtractContext(output)
		// Context before should be limited to 50 chars
		expectedPrefix := prefix[len(prefix)-50:]
		if len(result) < len(expectedPrefix)+len("MARKER") {
			t.Errorf("expected context to include 50 chars before + marker, got %q", result)
		}
	})

	t.Run("limits context to 50 chars after promise", func(t *testing.T) {
		d := New("MARKER")
		// Create output with more than 50 chars after marker
		suffix := "0123456789012345678901234567890123456789012345678901234567890"
		output := "MARKER" + suffix

		result := d.ExtractContext(output)
		// Context after should be limited to 50 chars
		expectedSuffix := suffix[:50]
		if len(result) < len("MARKER")+len(expectedSuffix) {
			t.Errorf("expected context to include marker + 50 chars after, got %q", result)
		}
	})

	t.Run("returns full context when output is short", func(t *testing.T) {
		d := New("OK")
		output := "status: OK done"

		result := d.ExtractContext(output)
		if result != output {
			t.Errorf("expected full output %q, got %q", output, result)
		}
	})

	t.Run("handles promise at start", func(t *testing.T) {
		d := New("START")
		output := "START of the message"

		result := d.ExtractContext(output)
		if result == "" {
			t.Error("expected non-empty context when promise at start")
		}
	})

	t.Run("handles promise at end", func(t *testing.T) {
		d := New("END")
		output := "message reaches the END"

		result := d.ExtractContext(output)
		if result == "" {
			t.Error("expected non-empty context when promise at end")
		}
	})

	t.Run("handles multiline context", func(t *testing.T) {
		d := New("TARGET")
		output := "line1\nline2\nTARGET\nline4\nline5"

		result := d.ExtractContext(output)
		if result == "" {
			t.Error("expected non-empty context for multiline output")
		}
	})

	t.Run("returns empty for empty output", func(t *testing.T) {
		d := New("ANY")

		result := d.ExtractContext("")
		if result != "" {
			t.Errorf("expected empty string for empty output, got %q", result)
		}
	})

	t.Run("extracts correct context with exact boundaries", func(t *testing.T) {
		d := New("X")
		// Exactly 50 chars before and after
		before := "01234567890123456789012345678901234567890123456789"
		after := "01234567890123456789012345678901234567890123456789"
		output := before + "X" + after

		result := d.ExtractContext(output)
		expected := before + "X" + after
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	})
}
