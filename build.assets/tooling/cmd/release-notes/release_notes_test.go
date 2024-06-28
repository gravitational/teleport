package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateReleaseNotes(t *testing.T) {
	testChangelog, err := os.Open(filepath.Join("testdata", "test-changelog.md"))
	require.NoError(t, err)
	expectedReleaseNotes, err := os.ReadFile(filepath.Join("testdata", "expected-release-notes.md"))
	require.NoError(t, err)

	gen := releaseNotesGenerator{}
	out, err := gen.generateReleaseNotes(testChangelog)
	require.NoError(t, err)
	require.Equal(t, string(expectedReleaseNotes), out)
}
