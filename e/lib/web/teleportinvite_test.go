package web

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsEmailLike(t *testing.T) {
	tests := []struct {
		name      string
		recipient string
		valid     bool
	}{
		{
			name:      "minimal valid case",
			recipient: "a@b",
			valid:     true,
		},
		{
			name:      "normal valid case",
			recipient: "alice@example.com",
			valid:     true,
		},
		{
			name:      "too many ats",
			recipient: "a@b@c",
			valid:     false,
		},
		{
			name:      "empty username",
			recipient: "@example.com",
			valid:     false,
		},
		{
			name:      "empty domain",
			recipient: "alice@",
			valid:     false,
		},
		{
			name:      "missing domain",
			recipient: "alice",
			valid:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.valid, isEmailLike(tc.recipient))
		})
	}
}
