// Package errors defines sentinel errors used across multiple packages.
package errors

import "errors"

// ErrBudgetExceeded is returned when the execution cost exceeds the configured maximum budget.
var ErrBudgetExceeded = errors.New("budget exceeded")

// ErrMaxIterationsReached is returned when the maximum number of iterations is reached without completion.
var ErrMaxIterationsReached = errors.New("max iterations reached")
