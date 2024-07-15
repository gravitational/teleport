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
	state, err := NewState(config)
	require.NoError(t, err)

	startTime, errt := state.GetStartTime()
	cursor, errc := state.GetCursor()
	id, erri := state.GetID()

	require.NoError(t, errt)
	require.NoError(t, errc)
	require.NoError(t, erri)

	assert.Nil(t, startTime)
	assert.Equal(t, "", cursor)
	assert.Equal(t, "", id)

	errc = state.SetCursor("testCursor")
	erri = state.SetID("testId")
	errt = state.SetStartTime(&currentTime)
	require.NoError(t, errc)
	require.NoError(t, erri)
	require.NoError(t, errt)

	state, err = NewState(config)
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
