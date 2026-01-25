package tui

// DefaultMaxOutputLines is the default maximum number of lines retained in the output buffer.
const DefaultMaxOutputLines = 10000

// RingBuffer is a fixed-size circular buffer for strings.
// When capacity is reached, new items overwrite the oldest items.
type RingBuffer struct {
	data  []string
	head  int  // Index of the oldest item
	count int  // Number of items in the buffer
	cap   int  // Maximum capacity
}

// NewRingBuffer creates a new RingBuffer with the specified capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = DefaultMaxOutputLines
	}
	return &RingBuffer{
		data: make([]string, capacity),
		cap:  capacity,
	}
}

// Push adds an item to the buffer, evicting the oldest if at capacity.
func (rb *RingBuffer) Push(item string) {
	if rb.count < rb.cap {
		// Buffer not full, append to the end
		idx := (rb.head + rb.count) % rb.cap
		rb.data[idx] = item
		rb.count++
	} else {
		// Buffer full, overwrite oldest
		rb.data[rb.head] = item
		rb.head = (rb.head + 1) % rb.cap
	}
}

// Len returns the number of items in the buffer.
func (rb *RingBuffer) Len() int {
	return rb.count
}

// Cap returns the maximum capacity of the buffer.
func (rb *RingBuffer) Cap() int {
	return rb.cap
}

// Get returns the item at the specified index (0 = oldest).
// Returns empty string if index is out of range.
func (rb *RingBuffer) Get(index int) string {
	if index < 0 || index >= rb.count {
		return ""
	}
	actualIdx := (rb.head + index) % rb.cap
	return rb.data[actualIdx]
}

// ToSlice returns all items as a slice, ordered from oldest to newest.
func (rb *RingBuffer) ToSlice() []string {
	if rb.count == 0 {
		return []string{}
	}

	result := make([]string, rb.count)
	for i := 0; i < rb.count; i++ {
		result[i] = rb.Get(i)
	}
	return result
}

// Clear removes all items from the buffer.
func (rb *RingBuffer) Clear() {
	rb.head = 0
	rb.count = 0
	// Clear references to allow GC
	for i := range rb.data {
		rb.data[i] = ""
	}
}

// Iterate calls fn for each item in the buffer, from oldest to newest.
// If fn returns false, iteration stops early.
func (rb *RingBuffer) Iterate(fn func(index int, item string) bool) {
	for i := 0; i < rb.count; i++ {
		actualIdx := (rb.head + i) % rb.cap
		if !fn(i, rb.data[actualIdx]) {
			return
		}
	}
}
