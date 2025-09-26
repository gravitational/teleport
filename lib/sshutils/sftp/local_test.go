package sftp

import (
	"os"
	"os/user"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandTildePrefix(t *testing.T) {
	origWd, err := os.Getwd()
	require.NoError(t, err)
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	wd, err := os.Getwd()
	require.NoError(t, err)
	currentUser, err := user.Current()
	require.NoError(t, err)
	home := currentUser.HomeDir

	tests := []struct {
		name     string
		input    string
		oldpwd   string
		expected string
	}{
		{
			name:     "no tilde",
			input:    "/path/to/foo",
			expected: "/path/to/foo",
		},
		{
			name:     "bare tilde",
			input:    "~",
			expected: home,
		},
		{
			name:     "tilde with path",
			input:    "~/foo/bar",
			expected: home + "/foo/bar",
		},
		{
			name:     "tilde with alternate user",
			input:    "~" + currentUser.Username + "/foo/bar",
			expected: home + "/foo/bar",
		},
		{
			name:     "tilde pwd",
			input:    "~+/foo/bar",
			expected: wd + "/foo/bar",
		},
		{
			name:     "tilde oldpwd",
			input:    "~-/foo/bar",
			oldpwd:   origWd,
			expected: origWd + "/foo/bar",
		},
		{
			name:     "tilde oldpwd unset",
			input:    "~-/foo/bar",
			expected: "~-/foo/bar",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.oldpwd != "" {
				t.Setenv("OLDPWD", tc.oldpwd)
			}
			expanded, err := expandTildePrefix(tc.input)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, expanded)
		})
	}
}
