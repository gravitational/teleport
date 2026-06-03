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

package events

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Event type string constants copied from lib/events/api.go (different Go module).
const (
	appSessionHTTPRequestEvent           = "app.session.http.request"
	appSessionHTTPRequestBodyChunkEvent  = "app.session.http.request.body_chunk"
	appSessionHTTPResponseEvent          = "app.session.http.response"
	appSessionHTTPResponseBodyChunkEvent = "app.session.http.response.body_chunk"
)

// TestAppSessionHTTPEventsRoundTrip verifies that all four HTTP recording event
// types can be wrapped via ToOneOf and trimmed via TrimToMaxSize.
func TestAppSessionHTTPEventsRoundTrip(t *testing.T) {
	t.Run("AppSessionHTTPRequest", func(t *testing.T) {
		ev := &AppSessionHTTPRequest{
			Metadata: Metadata{
				Type: appSessionHTTPRequestEvent,
			},
			RequestId:   "req-1",
			Method:      "GET",
			Url:         "https://example.com/api",
			HttpVersion: "HTTP/1.1",
			Headers: []*HTTPHeader{
				{Name: "Content-Type", Value: "application/json"},
			},
		}

		oneof, err := ToOneOf(ev)
		require.NoError(t, err)
		require.NotNil(t, oneof)

		trimmed := ev.TrimToMaxSize(4096)
		require.NotNil(t, trimmed)
	})

	t.Run("AppSessionHTTPRequestBodyChunk", func(t *testing.T) {
		ev := &AppSessionHTTPRequestBodyChunk{
			Metadata: Metadata{
				Type: appSessionHTTPRequestBodyChunkEvent,
			},
			RequestId:  "req-1",
			ChunkIndex: 0,
			IsLast:     true,
			Data:       []byte(`{"key":"value"}`),
		}

		oneof, err := ToOneOf(ev)
		require.NoError(t, err)
		require.NotNil(t, oneof)

		trimmed := ev.TrimToMaxSize(4096)
		require.NotNil(t, trimmed)
	})

	t.Run("AppSessionHTTPResponse", func(t *testing.T) {
		ev := &AppSessionHTTPResponse{
			Metadata: Metadata{
				Type: appSessionHTTPResponseEvent,
			},
			RequestId:   "req-1",
			StatusCode:  200,
			StatusText:  "OK",
			HttpVersion: "HTTP/1.1",
			Headers: []*HTTPHeader{
				{Name: "Content-Type", Value: "application/json"},
			},
		}

		oneof, err := ToOneOf(ev)
		require.NoError(t, err)
		require.NotNil(t, oneof)

		trimmed := ev.TrimToMaxSize(4096)
		require.NotNil(t, trimmed)
	})

	t.Run("AppSessionHTTPResponseBodyChunk", func(t *testing.T) {
		ev := &AppSessionHTTPResponseBodyChunk{
			Metadata: Metadata{
				Type: appSessionHTTPResponseBodyChunkEvent,
			},
			RequestId:  "req-1",
			ChunkIndex: 0,
			IsLast:     true,
			Data:       []byte(`{"result":"ok"}`),
		}

		oneof, err := ToOneOf(ev)
		require.NoError(t, err)
		require.NotNil(t, oneof)

		trimmed := ev.TrimToMaxSize(4096)
		require.NotNil(t, trimmed)
	})
}
