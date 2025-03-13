/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package sessionrecording_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/sessionrecording"
)

// TestReadCorruptedRecording tests that the streamer can successfully decode the kind of corrupted
// recordings that some older bugged versions of teleport might end up producing when under heavy load/throttling.
func TestReadCorruptedRecording(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f, err := os.Open("testdata/corrupted-session")
	require.NoError(t, err)
	defer f.Close()

	reader := sessionrecording.NewReader(f)
	defer reader.Close()

	events, err := reader.ReadAll(ctx)
	require.NoError(t, err)

	// verify that the expected number of events are extracted
	require.Len(t, events, 12)
}

// note: testdata/session is not a real session recording, but enough for
// demonstratation purposes
// testdata/session has just enough metadata in the protobuf binary file
// to have the event types populated
func ExampleNewReader() {
	// testdata/session includes 4 events
	// 1. session.start
	// 2. print
	// 3. session.end
	// 4. session.leave
	sessionFile, err := os.Open("testdata/session")
	if err != nil {
		// handle Open error
		return
	}
	defer sessionFile.Close()

	reader := sessionrecording.NewReader(sessionFile)
	defer reader.Close()

	events, err := reader.ReadAll(context.Background())
	if err != nil {
		// handle ReadAll error
		return
	}

	// loop through parsed events
	for _, event := range events {
		fmt.Println(event.GetType())
	}

	// Output: session.start
	// print
	// session.end
	// session.leave
}
