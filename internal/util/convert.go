// Package util provides shared utility functions used across the orbital CLI.
package util

import (
	"math"
	"strings"
)

// IntToString converts an integer to its string representation without
// using the fmt package. This is useful in hot paths where allocation
// from fmt.Sprintf should be avoided.
func IntToString(n int) string {
	if n == 0 {
		return "0"
	}
	// Handle math.MinInt specially: -math.MinInt overflows back to math.MinInt
	// because of two's complement representation
	if n == math.MinInt {
		// math.MinInt is -(2^63) for 64-bit, which is -9223372036854775808
		return "-9223372036854775808"
	}
	if n < 0 {
		return "-" + IntToString(-n)
	}
	var result strings.Builder
	for n > 0 {
		result.WriteString(string(rune('0' + n%10)))
		n /= 10
	}
	// Reverse the string
	s := result.String()
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// FormatNumber formats an integer with thousands separators (commas).
// For example, 1234567 becomes "1,234,567" and -1234567 becomes "-1,234,567".
func FormatNumber(n int) string {
	// Handle negative numbers by formatting the absolute value and prepending minus
	if n < 0 {
		// Use IntToString which handles math.MinInt correctly
		if n == math.MinInt {
			// math.MinInt formatted with commas
			return "-9,223,372,036,854,775,808"
		}
		return "-" + FormatNumber(-n)
	}

	s := IntToString(n)
	if len(s) <= 3 {
		return s
	}

	var result strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(r)
	}
	return result.String()
}
