// Package util provides shared utility functions used across the orbital CLI.
package util

import "strings"

// IntToString converts an integer to its string representation without
// using the fmt package. This is useful in hot paths where allocation
// from fmt.Sprintf should be avoided.
func IntToString(n int) string {
	if n == 0 {
		return "0"
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
// For example, 1234567 becomes "1,234,567".
func FormatNumber(n int) string {
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
