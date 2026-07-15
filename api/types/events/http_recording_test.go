/*
Copyright 2026 Gravitational, Inc.

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
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	appSessionHTTPRequestEvent           = "app.session.http.request"
	appSessionHTTPRequestBodyChunkEvent  = "app.session.http.request.body_chunk"
	appSessionHTTPResponseEvent          = "app.session.http.response"
	appSessionHTTPResponseBodyChunkEvent = "app.session.http.response.body_chunk"
)

// TestAppSessionHTTPEventsRoundTrip verifies that all four HTTP recording
// event types round-trip through OneOf conversion and proto serialization.
func TestAppSessionHTTPEventsRoundTrip(t *testing.T) {
	t.Run("AppSessionHTTPRequest", func(t *testing.T) {
		ev := &AppSessionHTTPRequest{
			Metadata: Metadata{
				Type: appSessionHTTPRequestEvent,
				Code: "T2015I",
			},
			RequestId:   "req-1",
			Method:      "POST",
			Url:         "https://example.com/v1/messages?x=1",
			HttpVersion: "HTTP/1.1",
			Headers: []*HTTPHeader{
				{Name: "Content-Type", Value: "application/json"},
			},
			RawQuery: "x=1",
		}
		assertOneOfRoundTrip(t, ev)
	})

	t.Run("AppSessionHTTPRequestBodyChunk", func(t *testing.T) {
		ev := &AppSessionHTTPRequestBodyChunk{
			Metadata: Metadata{
				Type: appSessionHTTPRequestBodyChunkEvent,
				Code: "T2016I",
			},
			RequestId:  "req-1",
			ChunkIndex: 2,
			IsLast:     true,
			Data:       []byte(`{"hello":"world"}`),
		}
		assertOneOfRoundTrip(t, ev)
	})

	t.Run("AppSessionHTTPResponse", func(t *testing.T) {
		ev := &AppSessionHTTPResponse{
			Metadata: Metadata{
				Type: appSessionHTTPResponseEvent,
				Code: "T2017I",
			},
			RequestId:   "req-1",
			StatusCode:  200,
			StatusText:  "OK",
			HttpVersion: "HTTP/1.1",
			Headers: []*HTTPHeader{
				{Name: "Content-Type", Value: "application/json"},
			},
			WaitTimeMs: 42,
		}
		assertOneOfRoundTrip(t, ev)
	})

	t.Run("AppSessionHTTPResponseBodyChunk", func(t *testing.T) {
		ev := &AppSessionHTTPResponseBodyChunk{
			Metadata: Metadata{
				Type: appSessionHTTPResponseBodyChunkEvent,
				Code: "T2018I",
			},
			RequestId:  "req-1",
			ChunkIndex: 0,
			IsLast:     true,
			Data:       []byte(`{"ok":true}`),
		}
		assertOneOfRoundTrip(t, ev)
	})
}

func assertOneOfRoundTrip(t *testing.T, ev AuditEvent) {
	t.Helper()

	oneOf, err := ToOneOf(ev)
	require.NoError(t, err)
	decoded, err := FromOneOf(*oneOf)
	require.NoError(t, err)
	require.Equal(t, ev, decoded)

	data, err := oneOf.Marshal()
	require.NoError(t, err)

	var restored OneOf
	require.NoError(t, restored.Unmarshal(data))
	decoded, err = FromOneOf(restored)
	require.NoError(t, err)
	require.Equal(t, ev, decoded)
}
