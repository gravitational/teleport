/*
Copyright 2015-2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"cmp"
	"log/slog"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	// osAndPort is teleport host and port
	osAndPort = "localhost:888"

	// currentTime is current time
	currentTime = time.Now().UTC().Truncate(time.Second)
)

func newStartCmdConfig(t *testing.T) *StartCmdConfig {
	storagePath := t.TempDir()

	return &StartCmdConfig{
		TeleportConfig: TeleportConfig{
			TeleportAddr: osAndPort,
		},
		IngestConfig: IngestConfig{
			StartTime:  &currentTime,
			StorageDir: storagePath,
		},
	}
}

// TestStatePersist checks that state is persisted when StartTime stays constant
func TestStatePersist(t *testing.T) {
	config := newStartCmdConfig(t)
	state, err := NewState(config, slog.Default())
	require.NoError(t, err)

	startTime, errt := state.GetStartTime()
	cursor, errc := state.GetCursor()
	id, erri := state.GetID()

	require.NoError(t, errt)
	require.NoError(t, errc)
	require.NoError(t, erri)

	assert.Nil(t, startTime)
	assert.Empty(t, cursor)
	assert.Empty(t, id)

	errc = state.SetCursor("testCursor")
	erri = state.SetID("testId")
	errt = state.SetStartTime(&currentTime)
	require.NoError(t, errc)
	require.NoError(t, erri)
	require.NoError(t, errt)

	state, err = NewState(config, slog.Default())
	require.NoError(t, err)

	startTime, errt = state.GetStartTime()
	require.NoError(t, errt)
	assert.NotNil(t, startTime)
	assert.Equal(t, currentTime, *startTime)

	cursor, errc = state.GetCursor()
	id, erri = state.GetID()

	require.NoError(t, errc)
	require.NoError(t, erri)

	assert.Equal(t, "testCursor", cursor)
	assert.Equal(t, "testId", id)
}

func TestStateMissingRecordings(t *testing.T) {
	config := newStartCmdConfig(t)
	state, err := NewState(config, slog.Default())
	require.NoError(t, err)

	// Iterating should find no records if nothing has been stored yet.
	err = state.IterateMissingRecordings(func(s session, attempts int) error {
		t.Errorf("no missing sessions should have been found, got %v", s)
		return nil
	})
	require.NoError(t, err)

	// Removing a missing item should not produce an error.
	err = state.RemoveMissingRecording("llama")
	require.NoError(t, err)

	expected := make([]session, 0, 10)
	for i := 0; i < 10; i++ {
		s := session{
			ID:         strconv.Itoa(i),
			Index:      int64(i),
			UploadTime: time.Now().UTC(),
		}
		err := state.SetMissingRecording(s, 0)
		expected = append(expected, s)
		assert.NoError(t, err)
	}

	// Iterating find all stored records and validate they match
	// the original sessions.
	records := make([]session, 0, 10)
	err = state.IterateMissingRecordings(func(s session, attempts int) error {
		records = append(records, s)
		return nil
	})
	require.NoError(t, err)

	slices.SortFunc(records, func(a, b session) int {
		return cmp.Compare(a.Index, b.Index)
	})

	require.EqualValues(t, expected, records)

	// Remove all items and validate they no longer exist.
	for i := 0; i < 10; i++ {
		err = state.RemoveMissingRecording(expected[i].ID)
		require.NoError(t, err)
	}

	err = state.IterateMissingRecordings(func(s session, attempts int) error {
		t.Errorf("no missing sessions should have been found, got %v", s)
		return nil
	})
	require.NoError(t, err)

}
