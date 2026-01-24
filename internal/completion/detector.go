// Package completion provides functionality for detecting task completion
// based on promise strings in command output.
package completion

import "strings"

// Detector checks for the presence of a promise string in output.
type Detector struct {
	promise string
}

// New creates a new Detector with the given promise string.
func New(promise string) *Detector {
	return &Detector{
		promise: promise,
	}
}

// Check returns true if the promise is found in the output.
// The match is case-sensitive and works with promise at any position
// in the output, including multiline output.
func (d *Detector) Check(output string) bool {
	return strings.Contains(output, d.promise)
}

// ExtractContext returns up to 50 characters before and after the promise
// in the output. Returns an empty string if the promise is not found.
func (d *Detector) ExtractContext(output string) string {
	idx := strings.Index(output, d.promise)
	if idx == -1 {
		return ""
	}

	// Calculate start position (up to 50 chars before promise)
	start := idx - 50
	if start < 0 {
		start = 0
	}

	// Calculate end position (up to 50 chars after promise)
	end := idx + len(d.promise) + 50
	if end > len(output) {
		end = len(output)
	}

	return output[start:end]
}
