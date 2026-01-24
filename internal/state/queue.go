package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Queue represents a queue of spec files waiting to be processed.
type Queue struct {
	QueuedFiles []string             `json:"queued_files"`
	AddedAt     map[string]time.Time `json:"added_at"`
	stateDir    string
}

// NewQueue creates a new empty Queue.
func NewQueue() *Queue {
	return &Queue{
		QueuedFiles: []string{},
		AddedAt:     make(map[string]time.Time),
	}
}

// LoadQueue loads the queue from queue.json in the state directory.
// Returns an empty queue if the file doesn't exist.
func LoadQueue(stateDir string) (*Queue, error) {
	queuePath := filepath.Join(stateDir, "queue.json")

	data, err := os.ReadFile(queuePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty queue if file doesn't exist
			q := NewQueue()
			q.stateDir = stateDir
			return q, nil
		}
		return nil, fmt.Errorf("failed to read queue file: %w", err)
	}

	var q Queue
	if err := json.Unmarshal(data, &q); err != nil {
		return nil, fmt.Errorf("failed to unmarshal queue: %w", err)
	}

	// Ensure maps are initialized
	if q.AddedAt == nil {
		q.AddedAt = make(map[string]time.Time)
	}
	if q.QueuedFiles == nil {
		q.QueuedFiles = []string{}
	}

	q.stateDir = stateDir
	return &q, nil
}

// save persists the queue to queue.json in the state directory.
func (q *Queue) save() error {
	if q.stateDir == "" {
		return fmt.Errorf("queue state directory not set")
	}

	queuePath := filepath.Join(q.stateDir, "queue.json")

	data, err := json.MarshalIndent(q, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal queue: %w", err)
	}

	// Write to temp file and rename for atomicity
	tempPath := queuePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write queue file: %w", err)
	}

	if err := os.Rename(tempPath, queuePath); err != nil {
		return fmt.Errorf("failed to rename queue file: %w", err)
	}

	return nil
}

// withLock executes the provided function while holding an exclusive lock on queue.json.
func (q *Queue) withLock(fn func() error) error {
	if q.stateDir == "" {
		return fmt.Errorf("queue state directory not set")
	}

	// Create lock file path
	lockPath := filepath.Join(q.stateDir, "queue.lock")

	// Open or create the lock file
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open lock file: %w", err)
	}
	defer func() {
		if err := lockFile.Close(); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: failed to close lock file: %v\n", err)
		}
	}()

	// Acquire exclusive lock
	if err := acquireLock(lockFile); err != nil {
		return err
	}
	defer func() {
		if err := releaseLock(lockFile); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
	}()

	// Reload queue from file to get latest state
	reloaded, err := LoadQueue(q.stateDir)
	if err != nil {
		return fmt.Errorf("failed to reload queue: %w", err)
	}

	// Update q's data with reloaded data
	q.QueuedFiles = reloaded.QueuedFiles
	q.AddedAt = reloaded.AddedAt

	// Execute the function
	if err := fn(); err != nil {
		return err
	}

	return nil
}

// Add adds a file to the queue with file locking for concurrent access protection.
// Duplicates are silently ignored (returns nil, no error).
func (q *Queue) Add(path string) error {
	return q.withLock(func() error {
		// Check for duplicates
		for _, f := range q.QueuedFiles {
			if f == path {
				// Silently ignore duplicates
				return nil
			}
		}

		q.QueuedFiles = append(q.QueuedFiles, path)
		q.AddedAt[path] = time.Now()

		return q.save()
	})
}

// Remove removes a file from the queue with file locking.
// Returns an error if the file is not in the queue.
func (q *Queue) Remove(path string) error {
	return q.withLock(func() error {
		found := false
		newFiles := make([]string, 0, len(q.QueuedFiles))
		for _, f := range q.QueuedFiles {
			if f == path {
				found = true
				continue
			}
			newFiles = append(newFiles, f)
		}

		if !found {
			return fmt.Errorf("file not found in queue: %s", path)
		}

		q.QueuedFiles = newFiles
		delete(q.AddedAt, path)

		return q.save()
	})
}

// Pop returns and clears all queued files.
// Returns an empty slice (not nil) if the queue is empty.
func (q *Queue) Pop() []string {
	// Copy files before clearing
	files := make([]string, len(q.QueuedFiles))
	copy(files, q.QueuedFiles)

	// Clear the queue
	q.QueuedFiles = []string{}
	q.AddedAt = make(map[string]time.Time)

	// Save the cleared state (ignore error for Pop simplicity)
	_ = q.save()

	return files
}

// IsEmpty returns true if the queue has no files.
func (q *Queue) IsEmpty() bool {
	return len(q.QueuedFiles) == 0
}

// Contains returns true if the queue contains the specified file.
func (q *Queue) Contains(path string) bool {
	for _, f := range q.QueuedFiles {
		if f == path {
			return true
		}
	}
	return false
}
