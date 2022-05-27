package tester

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestFormatSSOWarnings(t *testing.T) {
	tests := []struct {
		name        string
		description string
		info        *types.SSOWarnings
		want        string
	}{
		{
			name:        "empty",
			description: "",
			info:        nil,
			want:        "",
		},
		{
			name:        "message, no individual warnings",
			description: "my field",
			info:        &types.SSOWarnings{Message: "blah blah"},
			want:        "my field: blah blah\n",
		},
		{
			name:        "message and warnings",
			description: "my field",
			info: &types.SSOWarnings{
				Message:  "blah blah",
				Warnings: []string{"foo", "bar", "baz"},
			},
			want: "my field: blah blah. Warnings:\n  foo\n  bar\n  baz\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSSOWarnings(tt.description, tt.info)
			require.Equal(t, tt.want, got)
		})
	}
}
