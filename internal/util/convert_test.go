package util

import "testing"

func TestIntToString(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{"zero", 0, "0"},
		{"single digit", 5, "5"},
		{"double digit", 42, "42"},
		{"triple digit", 123, "123"},
		{"four digit", 1234, "1234"},
		{"large number", 1234567, "1234567"},
		{"negative single", -5, "-5"},
		{"negative multi", -123, "-123"},
		{"negative large", -1234567, "-1234567"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IntToString(tt.input)
			if result != tt.expected {
				t.Errorf("IntToString(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{"zero", 0, "0"},
		{"single digit", 5, "5"},
		{"double digit", 42, "42"},
		{"triple digit", 123, "123"},
		{"four digit", 1234, "1,234"},
		{"five digit", 12345, "12,345"},
		{"six digit", 123456, "123,456"},
		{"seven digit", 1234567, "1,234,567"},
		{"large number", 1234567890, "1,234,567,890"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatNumber(tt.input)
			if result != tt.expected {
				t.Errorf("FormatNumber(%d) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
