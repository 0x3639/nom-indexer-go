package indexer

import (
	"math/big"
	"testing"
)

func TestFormatArg(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "string input",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "byte slice",
			input:    []byte("world"),
			expected: "world",
		},
		{
			name:     "empty byte slice",
			input:    []byte{},
			expected: "",
		},
		{
			name:     "integer",
			input:    42,
			expected: "42",
		},
		{
			name:     "negative integer",
			input:    -100,
			expected: "-100",
		},
		{
			name:     "int64",
			input:    int64(9223372036854775807),
			expected: "9223372036854775807",
		},
		{
			name:     "uint64",
			input:    uint64(18446744073709551615),
			expected: "18446744073709551615",
		},
		{
			name:     "float64",
			input:    3.14159,
			expected: "3.14159",
		},
		{
			name:     "boolean true",
			input:    true,
			expected: "true",
		},
		{
			name:     "boolean false",
			input:    false,
			expected: "false",
		},
		{
			name:     "nil",
			input:    nil,
			expected: "<nil>",
		},
		{
			name:     "big.Int",
			input:    big.NewInt(1000000000000),
			expected: "1000000000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatArg(tt.input)
			if result != tt.expected {
				t.Errorf("formatArg(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
