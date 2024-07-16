/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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
			assert.Equal(t, tt.want, got)
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
