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

package export

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
)

// testState is a helper for easily/cleanly representing a cursor/exporter state as a literal when
// writing tests. the values of the inner mapping are generally strings, but some helpers support
// using []string for the completed key.
type testState map[string]map[string]any

// writeRawState writes the test state to a directory structure. unlike `newState`, this helper
// is designed to inject non-conforming data, so it will accept non-date dir names and expects a string
// value for the completed key.
func writeRawState(t *testing.T, dir string, ts testState) {
	for d, s := range ts {
		dateDir := filepath.Join(dir, d)
		require.NoError(t, os.MkdirAll(dateDir, teleport.SharedDirMode))

		for k, v := range s {
			fileName := filepath.Join(dateDir, k)
			require.NoError(t, os.WriteFile(fileName, []byte(v.(string)), 0644))
		}
	}
}

// newState converts a testState into a real ExporterState. this helper only works
// with a well-formed testState.
func newState(t *testing.T, ts testState) ExporterState {
	state := ExporterState{
		Dates: make(map[time.Time]DateExporterState),
	}
	for d, s := range ts {
		date, err := time.Parse(time.DateOnly, d)
		require.NoError(t, err)

		dateState := DateExporterState{
			Cursors:   make(map[string]string),
			Completed: []string{}, // avoids require.Equal rejecting nil slices as unequal to empty slices
		}

	Entries:
		for k, v := range s {
			if k == completedName {
				dateState.Completed = v.([]string)
				continue Entries
			}
			dateState.Cursors[k] = v.(string)
		}

		state.Dates[date] = dateState
	}
	return state
}

func syncAndVerifyState(t *testing.T, cursor *Cursor, ts testState) {
	state := newState(t, ts)

	// sync the state
	require.NoError(t, cursor.Sync(state))

	// verify in-memory state is as expected
	require.Equal(t, state, cursor.GetState())

	// attempt to load the state from disk
	loaded, err := NewCursor(CursorConfig{
		Dir: cursor.cfg.Dir,
	})
	require.NoError(t, err)

	// verify that the loaded state is the same as the original state
	require.Equal(t, state, loaded.GetState())
}

// verifyRawState asserts that the raw state on disk matches the provided test state. chunk names
// need to be suffixed to match and the completed key should be a string.
func verifyRawState(t *testing.T, dir string, ts testState) {
	for d, s := range ts {
		dateDir := filepath.Join(dir, d)
		for k, v := range s {
			fileName := filepath.Join(dateDir, k)
			data, err := os.ReadFile(fileName)
			require.NoError(t, err)

			require.Equal(t, v.(string), string(data))
		}
	}
}

// TestCursorBasics verifies basic syncing/loading of cursor state.
func TestCursorBasics(t *testing.T) {
	dir := t.TempDir()

	cursor, err := NewCursor(CursorConfig{
		Dir: dir,
	})
	require.NoError(t, err)
	state := cursor.GetState()
	require.True(t, state.IsEmpty())

	// sync and verify a typical state
	syncAndVerifyState(t, cursor, testState{
		"2021-01-01": {
			completedName: []string{"chunk1", "chunk2"},
		},
		"2021-01-02": {
			completedName: []string{"chunk1", "chunk2"},
			"chunk3":      "cursor1",
			"chunk4":      "cursor2",
		},
		"2021-01-03": {
			"chunk3": "cursor1",
			"chunk4": "cursor2",
		},
		"2021-01-04": {},
	})

	verifyRawState(t, dir, testState{
		"2021-01-01": {
			completedName: "chunk1\nchunk2\n",
		},
		"2021-01-02": {
			completedName:          "chunk1\nchunk2\n",
			"chunk3" + chunkSuffix: "cursor1",
			"chunk4" + chunkSuffix: "cursor2",
		},
		"2021-01-03": {
			"chunk3" + chunkSuffix: "cursor1",
			"chunk4" + chunkSuffix: "cursor2",
		},
		"2021-01-04": {},
	})

	// sync and verify updated state
	syncAndVerifyState(t, cursor, testState{
		"2021-01-01": {
			completedName: []string{"chunk1", "chunk2", "chunk3"},
		},
		"2021-01-02": {
			completedName: []string{"chunk1", "chunk2"},
			"chunk3":      "cursor1",
			"chunk4":      "cursor2",
			"chunk5":      "cursor4",
		},
		"2021-01-03": {
			"chunk3": "cursor2",
			"chunk4": "cursor3",
		},
		"2021-01-04": {},
		"2021-01-05": {},
	})

	verifyRawState(t, dir, testState{
		"2021-01-01": {
			completedName: "chunk1\nchunk2\nchunk3\n",
		},
		"2021-01-02": {
			completedName:          "chunk1\nchunk2\n",
			"chunk3" + chunkSuffix: "cursor1",
			"chunk4" + chunkSuffix: "cursor2",
			"chunk5" + chunkSuffix: "cursor4",
		},
		"2021-01-03": {
			"chunk3" + chunkSuffix: "cursor2",
			"chunk4" + chunkSuffix: "cursor3",
		},
		"2021-01-04": {},
		"2021-01-05": {},
	})

	// sync & verify heavily truncated state
	syncAndVerifyState(t, cursor, testState{
		"2021-01-05": {},
	})

	verifyRawState(t, dir, testState{
		"2021-01-05": {},
	})
}

func TestCursorBadState(t *testing.T) {
	dir := t.TempDir()

	writeRawState(t, dir, testState{
		"2021-01-01": {
			completedName: "\n\nchunk1\n\n", // extra whitespace should be ignored
		},
		"not-a-date": {
			completedName: "chunk1",
			"no-suffix":   "cursor1", // unknown suffix should be ignored
		},
		"2021-01-02": {
			completedName:          "\n", // whitespace-only completed file should count as empty
			"chunk1" + chunkSuffix: "cursor1",
			"chunk2" + chunkSuffix: "cursor2\n", // extra whitespace should be ignored
			"chunk3" + chunkSuffix: " ",         // whitespace-only cursor file should count as empty
			"no-suffix":            "cursor3",   // unknown suffix should be ignored
		},
	})

	expected := newState(t, testState{
		"2021-01-01": {
			completedName: []string{"chunk1"},
		},
		"2021-01-02": {
			"chunk1": "cursor1",
			"chunk2": "cursor2",
		},
	})

	loaded, err := NewCursor(CursorConfig{
		Dir: dir,
	})
	require.NoError(t, err)

	// verify that the loaded state is the same as expected state
	require.Equal(t, expected, loaded.GetState())
}
