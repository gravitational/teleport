package main

import (
	_ "embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_generateReleaseNotes(t *testing.T) {
	tests := []struct {
		name           string
		releaseVersion string
		clFile         *os.File
		want           string
		wantErr        bool
	}{
		{
			name:           "happy path",
			releaseVersion: "16.0.1",
			clFile:         mustOpen(t, "test-changelog.md"),
			want:           mustRead(t, "expected-release-notes.md"),
			wantErr:        false,
		},
		{
			name:           "version mismatch",
			releaseVersion: "15.0.1", // test-changelog has 16.0.1
			clFile:         mustOpen(t, "test-changelog.md"),
			want:           "",
			wantErr:        true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &releaseNotesGenerator{
				releaseVersion: tt.releaseVersion,
			}

			got, err := r.generateReleaseNotes(tt.clFile)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, got, tt.want)
		})
	}
}

func mustOpen(t *testing.T, filename string) *os.File {
	testfile, err := os.Open(filepath.Join("testdata", filename))
	require.NoError(t, err)
	return testfile
}

func mustRead(t *testing.T, filename string) string {
	expectedReleaseNotes, err := os.ReadFile(filepath.Join("testdata", filename))
	require.NoError(t, err)
	return string(expectedReleaseNotes)
}
