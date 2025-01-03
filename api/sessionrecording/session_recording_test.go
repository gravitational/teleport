/*
Copyright 2024 Gravitational, Inc.

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
