/*
Copyright 2020 Gravitational, Inc.

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

package events

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/gravitational/teleport/lib/session"

	"github.com/stretchr/testify/assert"
)

// TestProtoStreamer tests edge cases of proto streamer implementation
func TestProtoStreamer(t *testing.T) {
	type testCase struct {
		name           string
		minUploadBytes int64
		events         []AuditEvent
		err            error
	}
	testCases := []testCase{
		{
			name:           "5MB similar to S3 min size in bytes",
			minUploadBytes: 1024 * 1024 * 5,
			events:         GenerateTestSession(SessionParams{PrintEvents: 1}),
		},
		{
			name:           "get a part per message",
			minUploadBytes: 1,
			events:         GenerateTestSession(SessionParams{PrintEvents: 1}),
		},
		{
			name:           "small load test with some uneven numbers",
			minUploadBytes: 1024,
			events:         GenerateTestSession(SessionParams{PrintEvents: 1000}),
		},
		{
			name:           "no events",
			minUploadBytes: 1024*1024*5 + 64*1024,
		},
		{
			name:           "one event using the whole part",
			minUploadBytes: 1,
			events:         GenerateTestSession(SessionParams{PrintEvents: 0})[:1],
		},
	}

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uploader := NewMemoryUploader()
			streamer, err := NewProtoStreamer(ProtoStreamerConfig{
				Uploader:       uploader,
				MinUploadBytes: tc.minUploadBytes,
			})
			assert.Nil(t, err)

			sid := session.ID(fmt.Sprintf("test-%v", i))
			stream, err := streamer.CreateAuditStream(ctx, sid)
			assert.Nil(t, err)

			events := tc.events
			for _, event := range events {
				err := stream.EmitAuditEvent(ctx, event)
				if tc.err != nil {
					assert.IsType(t, tc.err, err)
					return
				}
				assert.Nil(t, err)
			}
			err = stream.Complete(ctx)
			assert.Nil(t, err)

			var outEvents []AuditEvent
			uploads, err := uploader.ListUploads(ctx)
			assert.Nil(t, err)
			parts, err := uploader.GetParts(uploads[0].ID)
			assert.Nil(t, err)

			for _, part := range parts {
				reader := NewProtoReader(bytes.NewReader(part))
				out, err := reader.ReadAll(ctx)
				assert.Nil(t, err, "part crash %#v", part)
				outEvents = append(outEvents, out...)
			}

			assert.Equal(t, events, outEvents)
		})
	}
}
