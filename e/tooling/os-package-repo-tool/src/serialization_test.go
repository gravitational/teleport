package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSafeLockName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		expectErr bool
	}{
		{
			name:     "basic",
			input:    "lockity.lock-123", // starts with letter, ends with number
			expected: "lockity.lock-123",
		},
		{
			name:     "no conversion",
			input:    "abcdefghijklm.nopqrstuvwxyz-1234567890",
			expected: "abcdefghijklm.nopqrstuvwxyz-1234567890",
		},
		{
			name:     "max length",
			input:    strings.Repeat("a", 63), // ends with letter
			expected: strings.Repeat("a", 63),
		},
		{
			name:     "ascii conversion",
			input:    "a!\"#$%&'()*+,/:;<=>?[\\]^_`{|}~a",
			expected: "a-----------------------------a",
		},
		{
			name:     "case conversion",
			input:    "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
			expected: "abcdefghijklmnopqrstuvwxyz",
		},
		{
			name:      "empty",
			input:     "",
			expectErr: true,
		},
		{
			name:      "too long",
			input:     strings.Repeat("a", 64),
			expectErr: true,
		},
		{
			name:      "does not start with letter",
			input:     "1lock",
			expectErr: true,
		},
		{
			name:      "contains unicode",
			input:     "eléphant",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := GetSafeLockName(tt.input)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, actual)
			}
		})
	}
}
