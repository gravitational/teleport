package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHasParentDir(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		parent     string
		wantResult bool
	}{
		{
			name:       "Has valid parent directory",
			path:       "/opt/teleport/dir/test",
			parent:     "/opt/teleport",
			wantResult: true,
		},
		{
			name:       "Has valid parent directory with slash",
			path:       "/opt/teleport/dir/test",
			parent:     "/opt/teleport/",
			wantResult: true,
		},
		{
			name:       "Parent directory is root",
			path:       "/opt/teleport/dir",
			parent:     "/",
			wantResult: true,
		},
		{
			name:       "Parent is the same as the path",
			path:       "/opt/teleport/dir",
			parent:     "/opt/teleport/dir",
			wantResult: false,
		},
		{
			name:       "Parent the same as the path but without slash",
			path:       "/opt/teleport/dir/",
			parent:     "/opt/teleport/dir",
			wantResult: false,
		},
		{
			name:       "Parent the same as the path but with slash",
			path:       "/opt/teleport/dir",
			parent:     "/opt/teleport/dir/",
			wantResult: false,
		},
		{
			name:       "Parent is substring of the path",
			path:       "/opt/teleport/dir-place",
			parent:     "/opt/teleport/dir",
			wantResult: false,
		},
		{
			name:       "Parent is in path",
			path:       "/opt/teleport",
			parent:     "/opt/teleport/dir",
			wantResult: false,
		},
		{
			name:       "Empty parent",
			path:       "/opt/teleport/dir",
			parent:     "",
			wantResult: false,
		},
		{
			name:       "Empty path",
			path:       "",
			parent:     "/opt/teleport",
			wantResult: false,
		},
		{
			name:       "Both empty",
			path:       "",
			parent:     "",
			wantResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := hasParentDir(tt.path, tt.parent)
			require.NoError(t, err)
			require.Equal(t, tt.wantResult, result)
		})
	}
}
