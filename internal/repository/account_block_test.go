package repository

import (
	"strings"
	"testing"
)

func TestSanitizeJSONForPostgres(t *testing.T) {
	// litNull is the literal 6-character JSON escape sequence for a NUL.
	// Use an escaped backslash so the literal survives source-to-file round trips.
	litNull := "\\u0000"

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty input unchanged",
			in:   "",
			want: "",
		},
		{
			name: "plain JSON object unchanged",
			in:   `{"a":1}`,
			want: `{"a":1}`,
		},
		{
			name: "literal escape sequence stripped",
			in:   `{"a":"` + litNull + `"}`,
			want: `{"a":""}`,
		},
		{
			name: "real null byte stripped",
			in:   "{\"a\":\"x\x00y\"}",
			want: `{"a":"xy"}`,
		},
		{
			name: "both forms stripped together",
			in:   litNull + "\x00",
			want: "",
		},
		{
			name: "multiple literal escapes stripped",
			in:   litNull + "abc" + litNull + "def" + litNull,
			want: "abcdef",
		},
		{
			name: "non-null unicode escape preserved",
			in:   `{"x":"é"}`,
			want: `{"x":"é"}`,
		},
		{
			name: "valid JSON with no nulls is byte-identical",
			in:   `{"hash":"abc","amount":42}`,
			want: `{"hash":"abc","amount":42}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeJSONForPostgres(tt.in)
			if got != tt.want {
				t.Errorf("sanitizeJSONForPostgres(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if strings.Contains(got, "\x00") {
				t.Errorf("output still contains a NUL byte: %q", got)
			}
			if strings.Contains(got, litNull) {
				t.Errorf("output still contains a literal %s escape: %q", litNull, got)
			}
		})
	}
}
