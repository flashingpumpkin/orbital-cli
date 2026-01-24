package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestGenerateSessionID tests that generateSessionID returns valid IDs.
func TestGenerateSessionID(t *testing.T) {
	id1, err := generateSessionID()
	if err != nil {
		t.Fatalf("generateSessionID failed: %v", err)
	}

	if len(id1) != 16 { // 8 bytes = 16 hex chars
		t.Errorf("expected 16 char ID, got %d", len(id1))
	}

	// Verify it's valid hex
	if !isValidSessionID(id1) {
		t.Errorf("generated ID is not valid: %s", id1)
	}

	// Generate another and ensure they're different
	id2, err := generateSessionID()
	if err != nil {
		t.Fatalf("generateSessionID failed: %v", err)
	}

	if id1 == id2 {
		t.Error("two generated IDs should not be equal")
	}
}

// TestIsValidSessionID tests session ID validation.
func TestIsValidSessionID(t *testing.T) {
	tests := []struct {
		id    string
		valid bool
	}{
		{"abc123", true},
		{"ABC123", true},
		{"aB1cD2", true},
		{"", false},
		{"../etc/passwd", false},
		{"session/output", false},
		{"session%20id", false},
		{"session id", false},
		{string(make([]byte, 65)), false}, // Too long
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := isValidSessionID(tt.id); got != tt.valid {
				t.Errorf("isValidSessionID(%q) = %v, want %v", tt.id, got, tt.valid)
			}
		})
	}
}

// TestRingBuffer tests the ring buffer implementation.
func TestRingBuffer(t *testing.T) {
	rb := NewRingBuffer(3)

	// Test empty buffer
	items := rb.ReadAll()
	if len(items) != 0 {
		t.Errorf("expected empty buffer, got %d items", len(items))
	}

	// Add items
	rb.Write(OutputMsg{Type: "text", Content: "1"})
	rb.Write(OutputMsg{Type: "text", Content: "2"})

	items = rb.ReadAll()
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
	if items[0].Content != "1" || items[1].Content != "2" {
		t.Error("items not in correct order")
	}

	// Fill buffer and wrap
	rb.Write(OutputMsg{Type: "text", Content: "3"})
	rb.Write(OutputMsg{Type: "text", Content: "4"}) // Should overwrite "1"

	items = rb.ReadAll()
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
	if items[0].Content != "2" || items[1].Content != "3" || items[2].Content != "4" {
		t.Errorf("items not in correct order after wrap: %v", items)
	}
}

// TestRingBufferConcurrent tests ring buffer thread safety.
func TestRingBufferConcurrent(t *testing.T) {
	rb := NewRingBuffer(100)
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				rb.Write(OutputMsg{Type: "text", Content: "msg"})
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				rb.ReadAll()
			}
		}()
	}

	wg.Wait()
	// If we get here without deadlock/panic, test passes
}

// TestSessionClone tests that Clone creates an independent copy.
func TestSessionClone(t *testing.T) {
	original := &Session{
		ID:            "test123",
		SpecFiles:     []string{"spec1.md", "spec2.md"},
		Status:        StatusRunning,
		WorkingDir:    "/tmp/test",
		Iteration:     5,
		MaxIterations: 50,
		TotalCost:     10.5,
		MaxBudget:     100.0,
		StartedAt:     time.Now(),
		ContextFiles:  []string{"ctx1.md"},
		Worktree: &WorktreeInfo{
			Name:   "test-worktree",
			Path:   "/tmp/worktree",
			Branch: "feature-branch",
		},
	}

	clone := original.Clone()

	// Verify values are copied
	if clone.ID != original.ID {
		t.Error("ID not cloned")
	}
	if clone.Status != original.Status {
		t.Error("Status not cloned")
	}
	if clone.Iteration != original.Iteration {
		t.Error("Iteration not cloned")
	}

	// Verify slices are independent
	clone.SpecFiles[0] = "modified.md"
	if original.SpecFiles[0] == "modified.md" {
		t.Error("SpecFiles slice not independent")
	}

	clone.ContextFiles[0] = "modified-ctx.md"
	if original.ContextFiles[0] == "modified-ctx.md" {
		t.Error("ContextFiles slice not independent")
	}

	// Verify worktree is independent
	clone.Worktree.Name = "modified-worktree"
	if original.Worktree.Name == "modified-worktree" {
		t.Error("Worktree not independent")
	}
}

// TestSessionCloseDone tests that CloseDone is safe to call multiple times.
func TestSessionCloseDone(t *testing.T) {
	session := &Session{
		ID:   "test123",
		done: make(chan struct{}),
	}

	// Should not panic when called multiple times
	session.CloseDone()
	session.CloseDone()
	session.CloseDone()

	// Verify channel is closed
	select {
	case <-session.done:
		// Good, channel is closed
	default:
		t.Error("done channel should be closed")
	}
}

// TestRegistryAddAndGet tests basic registry operations.
func TestRegistryAddAndGet(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	session := &Session{
		ID:        "test123",
		SpecFiles: []string{"spec.md"},
		Status:    StatusRunning,
	}

	// Add session
	if err := r.Add(session); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	// Get session
	got, exists := r.Get("test123")
	if !exists {
		t.Fatal("session not found")
	}
	if got.ID != "test123" {
		t.Errorf("got ID %s, want test123", got.ID)
	}

	// Duplicate add should fail
	if err := r.Add(session); err == nil {
		t.Error("duplicate add should fail")
	}

	// Get non-existent
	_, exists = r.Get("nonexistent")
	if exists {
		t.Error("non-existent session should not be found")
	}
}

// TestRegistryUpdateStatus tests status updates.
func TestRegistryUpdateStatus(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	session := &Session{
		ID:     "test123",
		Status: StatusRunning,
	}
	r.Add(session)

	// Update to completed
	if err := r.UpdateStatus("test123", StatusCompleted, ""); err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	got, _ := r.Get("test123")
	if got.Status != StatusCompleted {
		t.Errorf("status = %s, want %s", got.Status, StatusCompleted)
	}
	if got.CompletedAt == nil {
		t.Error("CompletedAt should be set for terminal status")
	}

	// Verify done channel is closed
	internal, _ := r.GetInternal("test123")
	select {
	case <-internal.done:
		// Good
	default:
		t.Error("done channel should be closed after terminal status")
	}
}

// TestRegistryPersistence tests save and load.
func TestRegistryPersistence(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	session := &Session{
		ID:            "test123",
		SpecFiles:     []string{"spec.md"},
		Status:        StatusCompleted,
		TotalCost:     25.5,
		Model:         "opus",
		CheckerModel:  "haiku",
		WorkflowName:  "spec-driven",
		MaxIterations: 50,
	}
	r.Add(session)
	r.UpdateStatus("test123", StatusCompleted, "")

	// Create new registry and load
	r2 := NewRegistry(dir)
	if err := r2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	got, exists := r2.Get("test123")
	if !exists {
		t.Fatal("session not found after load")
	}
	if got.TotalCost != 25.5 {
		t.Errorf("TotalCost = %f, want 25.5", got.TotalCost)
	}
	if got.Model != "opus" {
		t.Errorf("Model = %s, want opus", got.Model)
	}
}

// TestRegistryLoadInterruptedSessions tests that running sessions are marked interrupted on load.
func TestRegistryLoadInterruptedSessions(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, ".orbital", "daemon-state.json")
	os.MkdirAll(filepath.Dir(stateFile), 0755)

	// Write state with running session
	state := struct {
		Sessions map[string]*Session `json:"sessions"`
	}{
		Sessions: map[string]*Session{
			"running1": {ID: "running1", Status: StatusRunning},
			"merging1": {ID: "merging1", Status: StatusMerging},
			"done1":    {ID: "done1", Status: StatusCompleted},
		},
	}
	data, _ := json.Marshal(state)
	os.WriteFile(stateFile, data, 0644)

	// Load registry
	r := NewRegistry(dir)
	if err := r.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check running was marked interrupted
	s1, _ := r.Get("running1")
	if s1.Status != StatusInterrupted {
		t.Errorf("running session status = %s, want %s", s1.Status, StatusInterrupted)
	}

	s2, _ := r.Get("merging1")
	if s2.Status != StatusInterrupted {
		t.Errorf("merging session status = %s, want %s", s2.Status, StatusInterrupted)
	}

	// Completed should remain completed
	s3, _ := r.Get("done1")
	if s3.Status != StatusCompleted {
		t.Errorf("completed session status = %s, want %s", s3.Status, StatusCompleted)
	}
}

// TestRegistryCorruptRecovery tests recovery from corrupt state file.
func TestRegistryCorruptRecovery(t *testing.T) {
	dir := t.TempDir()
	stateFile := filepath.Join(dir, ".orbital", "daemon-state.json")
	backupFile := stateFile + ".bak"
	os.MkdirAll(filepath.Dir(stateFile), 0755)

	// Write valid backup
	state := struct {
		Sessions map[string]*Session `json:"sessions"`
	}{
		Sessions: map[string]*Session{
			"backup1": {ID: "backup1", Status: StatusCompleted},
		},
	}
	data, _ := json.Marshal(state)
	os.WriteFile(backupFile, data, 0644)

	// Write corrupt main file
	os.WriteFile(stateFile, []byte("{corrupt json"), 0644)

	// Load should recover from backup
	r := NewRegistry(dir)
	if err := r.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	_, exists := r.Get("backup1")
	if !exists {
		t.Error("session from backup not found")
	}
}

// TestRegistrySubscribeBroadcast tests the pub/sub mechanism.
func TestRegistrySubscribeBroadcast(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	session := &Session{
		ID:     "test123",
		Status: StatusRunning,
	}
	r.Add(session)

	// Subscribe
	ch, history, done, err := r.Subscribe("test123")
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	defer r.Unsubscribe("test123", ch)

	if len(history) != 0 {
		t.Errorf("expected empty history, got %d items", len(history))
	}

	// Broadcast message
	msg := OutputMsg{Type: "text", Content: "hello"}
	r.Broadcast("test123", msg)

	// Should receive message
	select {
	case received := <-ch:
		if received.Content != "hello" {
			t.Errorf("received content = %s, want hello", received.Content)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for broadcast")
	}

	// New subscriber should get history
	ch2, history2, _, _ := r.Subscribe("test123")
	defer r.Unsubscribe("test123", ch2)

	if len(history2) != 1 {
		t.Errorf("expected 1 history item, got %d", len(history2))
	}

	// Verify done channel works
	r.UpdateStatus("test123", StatusCompleted, "")
	select {
	case <-done:
		// Good
	case <-time.After(time.Second):
		t.Error("done channel should be closed")
	}
}

// TestSessionRunnerConcurrentLimit tests the concurrent session limit.
func TestSessionRunnerConcurrentLimit(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	cfg := &DaemonConfig{
		MaxConcurrentSessions: 2,
		DefaultWorkflow:       "spec-driven",
	}
	runner := NewSessionRunner(r, dir, cfg)

	// Manually add sessions to cancels map to simulate running sessions
	runner.mu.Lock()
	runner.cancels["session1"] = func() {}
	runner.cancels["session2"] = func() {}
	runner.mu.Unlock()

	// Third session should fail
	_, err := runner.Start(context.Background(), StartSessionRequest{
		SpecFiles:     []string{"spec.md"},
		MaxIterations: 50,
	})

	if err == nil {
		t.Error("expected error for exceeding concurrent limit")
	}
}

// TestDaemonConfig tests default config values.
func TestDaemonConfig(t *testing.T) {
	cfg := DefaultDaemonConfig()

	if cfg.DefaultBudget != 50.0 {
		t.Errorf("DefaultBudget = %f, want 50.0", cfg.DefaultBudget)
	}
	if cfg.DefaultWorkflow != "spec-driven" {
		t.Errorf("DefaultWorkflow = %s, want spec-driven", cfg.DefaultWorkflow)
	}
	if cfg.ChatModel != "sonnet" {
		t.Errorf("ChatModel = %s, want sonnet", cfg.ChatModel)
	}
}

// TestRegistryConcurrent tests concurrent registry access.
func TestRegistryConcurrent(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	// Add initial session
	r.Add(&Session{ID: "test1", Status: StatusRunning})

	var wg sync.WaitGroup

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				r.Get("test1")
				r.List()
				r.Count()
			}
		}()
	}

	// Concurrent writes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				r.UpdateProgress("test1", j, float64(j)*0.1, j*100, j*50)
			}
		}(i)
	}

	// Concurrent broadcasts
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				r.Broadcast("test1", OutputMsg{Type: "text", Content: "msg"})
			}
		}()
	}

	wg.Wait()
	// If we get here without deadlock/panic, test passes
}
