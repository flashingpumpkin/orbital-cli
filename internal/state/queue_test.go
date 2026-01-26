package state

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/flashingpumpkin/orbital/internal/testhelpers"
)

func TestQueue_NewQueueReturnsEmptyQueue(t *testing.T) {
	q := NewQueue()

	if !q.IsEmpty() {
		t.Error("NewQueue() should return empty queue")
	}
	if len(q.QueuedFiles) != 0 {
		t.Errorf("QueuedFiles length = %d; want 0", len(q.QueuedFiles))
	}
	if len(q.AddedAt) != 0 {
		t.Errorf("AddedAt length = %d; want 0", len(q.AddedAt))
	}
}

func TestQueue_LoadQueue_ReturnsEmptyQueueWhenNoFile(t *testing.T) {
	_, stateDir := testhelpers.StateDir(t)

	q, err := LoadQueue(stateDir)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}

	if !q.IsEmpty() {
		t.Error("LoadQueue() should return empty queue when no file exists")
	}
}

func TestQueue_SaveAndLoad_RoundTrip(t *testing.T) {
	_, stateDir := testhelpers.StateDir(t)

	// Create queue with files
	original := NewQueue()
	original.stateDir = stateDir
	original.QueuedFiles = []string{"/path/to/spec1.md", "/path/to/spec2.md"}
	original.AddedAt = map[string]time.Time{
		"/path/to/spec1.md": time.Now().Add(-10 * time.Minute),
		"/path/to/spec2.md": time.Now().Add(-5 * time.Minute),
	}

	err := original.save()
	if err != nil {
		t.Fatalf("save() error = %v", err)
	}

	// Load queue
	loaded, err := LoadQueue(stateDir)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}

	if len(loaded.QueuedFiles) != 2 {
		t.Errorf("QueuedFiles length = %d; want 2", len(loaded.QueuedFiles))
	}
	if loaded.QueuedFiles[0] != "/path/to/spec1.md" {
		t.Errorf("QueuedFiles[0] = %q; want %q", loaded.QueuedFiles[0], "/path/to/spec1.md")
	}
}

func TestQueue_Add_AddsFileToQueue(t *testing.T) {
	_, stateDir := testhelpers.StateDir(t)

	q, err := LoadQueue(stateDir)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}

	err = q.Add("/path/to/spec.md")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if q.IsEmpty() {
		t.Error("queue should not be empty after Add()")
	}
	if len(q.QueuedFiles) != 1 {
		t.Errorf("QueuedFiles length = %d; want 1", len(q.QueuedFiles))
	}
	if q.QueuedFiles[0] != "/path/to/spec.md" {
		t.Errorf("QueuedFiles[0] = %q; want %q", q.QueuedFiles[0], "/path/to/spec.md")
	}
	if _, ok := q.AddedAt["/path/to/spec.md"]; !ok {
		t.Error("AddedAt should contain the added file")
	}
}

func TestQueue_Add_DuplicateSilentlyIgnored(t *testing.T) {
	_, stateDir := testhelpers.StateDir(t)

	q, err := LoadQueue(stateDir)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}

	// Add same file twice
	err = q.Add("/path/to/spec.md")
	if err != nil {
		t.Fatalf("first Add() error = %v", err)
	}

	err = q.Add("/path/to/spec.md")
	if err != nil {
		t.Fatalf("second Add() error = %v", err)
	}

	// Should still have only one entry
	if len(q.QueuedFiles) != 1 {
		t.Errorf("QueuedFiles length = %d; want 1 (duplicates should be ignored)", len(q.QueuedFiles))
	}
}

func TestQueue_Remove_RemovesFileFromQueue(t *testing.T) {
	_, stateDir := testhelpers.StateDir(t)

	q, err := LoadQueue(stateDir)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}
	if err := q.Add("/path/to/spec.md"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	err = q.Remove("/path/to/spec.md")
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	if !q.IsEmpty() {
		t.Error("queue should be empty after removing only file")
	}
	if _, ok := q.AddedAt["/path/to/spec.md"]; ok {
		t.Error("AddedAt should not contain removed file")
	}
}

func TestQueue_Remove_ReturnsErrorWhenFileNotFound(t *testing.T) {
	_, stateDir := testhelpers.StateDir(t)

	q, err := LoadQueue(stateDir)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}

	err = q.Remove("/nonexistent.md")
	if err == nil {
		t.Error("Remove() should return error for nonexistent file")
	}
}

func TestQueue_Pop_ReturnsAndClearsAllFiles(t *testing.T) {
	_, stateDir := testhelpers.StateDir(t)

	q, err := LoadQueue(stateDir)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}
	if err := q.Add("/path/to/spec1.md"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := q.Add("/path/to/spec2.md"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	files, err := q.Pop()
	if err != nil {
		t.Fatalf("Pop() returned unexpected error: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("Pop() returned %d files; want 2", len(files))
	}
	if !q.IsEmpty() {
		t.Error("queue should be empty after Pop()")
	}
	if len(q.AddedAt) != 0 {
		t.Error("AddedAt should be empty after Pop()")
	}
}

func TestQueue_Pop_ReturnsEmptySliceWhenEmpty(t *testing.T) {
	_, stateDir := testhelpers.StateDir(t)

	q, err := LoadQueue(stateDir)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}

	files, err := q.Pop()
	if err != nil {
		t.Fatalf("Pop() returned unexpected error: %v", err)
	}

	if files == nil {
		t.Error("Pop() should return empty slice, not nil")
	}
	if len(files) != 0 {
		t.Errorf("Pop() returned %d files; want 0", len(files))
	}
}

func TestQueue_Contains_ReturnsTrueForQueuedFile(t *testing.T) {
	_, stateDir := testhelpers.StateDir(t)

	q, err := LoadQueue(stateDir)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}
	if err := q.Add("/path/to/spec.md"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if !q.Contains("/path/to/spec.md") {
		t.Error("Contains() = false; want true for queued file")
	}
}

func TestQueue_Contains_ReturnsFalseForNonQueuedFile(t *testing.T) {
	_, stateDir := testhelpers.StateDir(t)

	q, err := LoadQueue(stateDir)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}

	if q.Contains("/nonexistent.md") {
		t.Error("Contains() = true; want false for non-queued file")
	}
}

func TestQueue_IsEmpty_ReturnsTrueWhenEmpty(t *testing.T) {
	q := NewQueue()

	if !q.IsEmpty() {
		t.Error("IsEmpty() = false; want true for empty queue")
	}
}

func TestQueue_IsEmpty_ReturnsFalseWhenNotEmpty(t *testing.T) {
	_, stateDir := testhelpers.StateDir(t)

	q, err := LoadQueue(stateDir)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}
	if err := q.Add("/path/to/spec.md"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if q.IsEmpty() {
		t.Error("IsEmpty() = true; want false for non-empty queue")
	}
}

func TestQueue_ConcurrentAdd_DoesNotCorrupt(t *testing.T) {
	_, stateDir := testhelpers.StateDir(t)

	// Run multiple concurrent adds
	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Use error channel to collect errors from goroutines (t.Errorf is not safe to call from goroutines)
	errCh := make(chan error, numGoroutines*2)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			// Each goroutine loads its own queue instance
			q, err := LoadQueue(stateDir)
			if err != nil {
				errCh <- err
				return
			}
			err = q.Add("/path/to/spec" + string(rune('A'+idx)) + ".md")
			if err != nil {
				errCh <- err
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	// Report any errors collected from goroutines
	for err := range errCh {
		t.Errorf("goroutine error: %v", err)
	}

	// Load final queue and verify integrity
	finalQ, err := LoadQueue(stateDir)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}

	// Should have exactly numGoroutines entries
	if len(finalQ.QueuedFiles) != numGoroutines {
		t.Errorf("QueuedFiles length = %d; want %d (concurrent adds may have corrupted)", len(finalQ.QueuedFiles), numGoroutines)
	}

	// Verify AddedAt entries match QueuedFiles
	if len(finalQ.AddedAt) != len(finalQ.QueuedFiles) {
		t.Errorf("AddedAt length = %d; QueuedFiles length = %d; should match", len(finalQ.AddedAt), len(finalQ.QueuedFiles))
	}

	for _, f := range finalQ.QueuedFiles {
		if _, ok := finalQ.AddedAt[f]; !ok {
			t.Errorf("AddedAt missing entry for %q", f)
		}
	}
}

func TestQueue_Add_PersistsToFile(t *testing.T) {
	_, stateDir := testhelpers.StateDir(t)

	q1, err := LoadQueue(stateDir)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}
	if err := q1.Add("/path/to/spec.md"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Load from a new queue instance
	q2, err := LoadQueue(stateDir)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}

	if !q2.Contains("/path/to/spec.md") {
		t.Error("Add() should persist to file, but new queue instance doesn't contain added file")
	}
}

func TestQueue_Remove_PersistsToFile(t *testing.T) {
	_, stateDir := testhelpers.StateDir(t)

	q1, err := LoadQueue(stateDir)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}
	if err := q1.Add("/path/to/spec.md"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := q1.Remove("/path/to/spec.md"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	// Load from a new queue instance
	q2, err := LoadQueue(stateDir)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}

	if q2.Contains("/path/to/spec.md") {
		t.Error("Remove() should persist to file, but new queue instance still contains removed file")
	}
}

func TestQueue_Pop_PersistsToFile(t *testing.T) {
	_, stateDir := testhelpers.StateDir(t)

	q1, err := LoadQueue(stateDir)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}
	if err := q1.Add("/path/to/spec.md"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	_, err = q1.Pop()
	if err != nil {
		t.Fatalf("Pop() returned unexpected error: %v", err)
	}

	// Load from a new queue instance
	q2, err := LoadQueue(stateDir)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}

	if !q2.IsEmpty() {
		t.Error("Pop() should persist to file, but new queue instance is not empty")
	}
}

func TestQueue_Pop_ReturnsErrorWhenSaveFails(t *testing.T) {
	_, stateDir := testhelpers.StateDir(t)

	q, err := LoadQueue(stateDir)
	if err != nil {
		t.Fatalf("LoadQueue() error = %v", err)
	}
	if err := q.Add("/path/to/spec.md"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Make the state directory read-only to cause save to fail
	if err := os.Chmod(stateDir, 0555); err != nil {
		t.Fatalf("failed to chmod state dir: %v", err)
	}
	// Restore permissions after test
	defer func() {
		_ = os.Chmod(stateDir, 0755)
	}()

	files, err := q.Pop()

	// Pop should still return the files (they were copied before save attempt)
	if len(files) != 1 {
		t.Errorf("Pop() returned %d files; want 1", len(files))
	}
	// But it should also return an error
	if err == nil {
		t.Error("Pop() should return error when save fails")
	}
}
