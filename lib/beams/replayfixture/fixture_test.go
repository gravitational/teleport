/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package replayfixture

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

const (
	testBeamID = "beam-1"
	otherBeam  = "beam-2"
)

var testStart = time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)

func chunkEvent(id, beamID string, at time.Time) *apievents.AppSessionChunk {
	return &apievents.AppSessionChunk{
		Metadata:       apievents.Metadata{Type: events.AppSessionChunkEvent, Time: at},
		UserMetadata:   apievents.UserMetadata{BeamID: beamID},
		SessionChunkID: id,
	}
}

func requestEvent(reqID, url string, at time.Time, headers ...*apievents.HTTPHeader) *apievents.AppSessionHTTPRequest {
	return &apievents.AppSessionHTTPRequest{
		Metadata:  apievents.Metadata{Type: events.AppSessionHTTPRequestEvent, Time: at},
		RequestId: reqID,
		Method:    "POST",
		Url:       url,
		Headers:   headers,
	}
}

func responseEvent(reqID string, status uint32, at time.Time) *apievents.AppSessionHTTPResponse {
	return &apievents.AppSessionHTTPResponse{
		Metadata:   apievents.Metadata{Type: events.AppSessionHTTPResponseEvent, Time: at},
		RequestId:  reqID,
		StatusCode: status,
	}
}

func requestChunk(reqID string, index int64, data string, isLast bool) *apievents.AppSessionHTTPRequestBodyChunk {
	return &apievents.AppSessionHTTPRequestBodyChunk{
		Metadata:   apievents.Metadata{Type: events.AppSessionHTTPRequestBodyChunkEvent, Time: testStart},
		RequestId:  reqID,
		ChunkIndex: index,
		IsLast:     isLast,
		Data:       []byte(data),
	}
}

func responseChunk(reqID string, index int64, data string, isLast bool) *apievents.AppSessionHTTPResponseBodyChunk {
	return &apievents.AppSessionHTTPResponseBodyChunk{
		Metadata:   apievents.Metadata{Type: events.AppSessionHTTPResponseBodyChunkEvent, Time: testStart},
		RequestId:  reqID,
		ChunkIndex: index,
		IsLast:     isLast,
		Data:       []byte(data),
	}
}

// testFixture builds a fixture with two chunk recordings. Exchange "r1" has its
// bodies split across both recordings (the case a fixture has to survive:
// request IDs span recordings), and "r2" is recorded without a response
// envelope, as an export clipped mid-flight would be.
func testFixture(t *testing.T) *Fixture {
	t.Helper()

	beamEvents := []apievents.AuditEvent{
		chunkEvent("chunk-1", testBeamID, testStart),
		chunkEvent("chunk-2", testBeamID, testStart.Add(time.Minute)),
		// Not attributed to the beam under export: SearchEvents must filter it out.
		chunkEvent("chunk-other", otherBeam, testStart),
	}
	chunks := map[string][]apievents.AuditEvent{
		"chunk-1": {
			requestEvent("r1", "https://api.anthropic.com/v1/messages", testStart,
				&apievents.HTTPHeader{Name: "X-Codex-Turn-Metadata", Value: "first"},
				&apievents.HTTPHeader{Name: "x-codex-turn-metadata", Value: "second"},
			),
			requestChunk("r1", 0, "req-a", false),
			requestChunk("r1", 1, "req-b", true),
			responseChunk("r1", 0, "resp-1", false),
			requestEvent("r2", "https://api.anthropic.com/v1/messages", testStart.Add(2*time.Minute)),
			requestChunk("r2", 0, "clipped", true),
		},
		"chunk-2": {
			responseChunk("r1", 1, "resp-2", true),
			responseEvent("r1", 200, testStart.Add(time.Second)),
		},
	}

	f, err := New(JobParams{
		BeamID:    testBeamID,
		Alias:     "curious-otter",
		Owner:     "alice",
		AppName:   "claude",
		CreatedAt: testStart,
		ExpiresAt: testStart.Add(time.Hour),
	}, beamEvents, chunks)
	require.NoError(t, err)
	return f
}

// TestRoundTrip asserts a fixture survives the trip through a file: the job
// metadata, the beam events, and the chunk recordings all come back.
func TestRoundTrip(t *testing.T) {
	f := testFixture(t)

	path := filepath.Join(t.TempDir(), "beam.json")
	require.NoError(t, f.WriteFile(path))

	loaded, err := LoadFile(path)
	require.NoError(t, err)
	require.Equal(t, f.Job, loaded.Job)
	require.Len(t, loaded.Events, 3)
	require.Len(t, loaded.Chunks, 2)

	// Decoding returns typed events again, with their payloads intact.
	beamEvents, err := loaded.AuditEvents()
	require.NoError(t, err)
	require.Len(t, beamEvents, 3)
	first, ok := beamEvents[0].(*apievents.AppSessionChunk)
	require.True(t, ok, "expected an app session chunk event, got %T", beamEvents[0])
	require.Equal(t, "chunk-1", first.SessionChunkID)
	require.Equal(t, testBeamID, first.GetBeamID())

	chunkEvents, err := loaded.ChunkEvents("chunk-1")
	require.NoError(t, err)
	require.Len(t, chunkEvents, 6)
	body, ok := chunkEvents[1].(*apievents.AppSessionHTTPRequestBodyChunk)
	require.True(t, ok, "expected a request body chunk, got %T", chunkEvents[1])
	require.Equal(t, []byte("req-a"), body.Data)

	_, err = loaded.ChunkEvents("nope")
	require.True(t, err != nil && trace.IsNotFound(err), "expected NotFound, got %v", err)
}

// TestInferLLMFormat asserts the format is derived from the recorded traffic,
// since the replay manifest an export is built from does not carry it.
func TestInferLLMFormat(t *testing.T) {
	t.Run("from anthropic request path", func(t *testing.T) {
		require.Equal(t, types.LLMFormatAnthropic, testFixture(t).Job.LLMFormat)
	})

	t.Run("from openai request path", func(t *testing.T) {
		f, err := New(JobParams{BeamID: testBeamID}, nil, map[string][]apievents.AuditEvent{
			"chunk-1": {requestEvent("r1", "https://api.openai.com/v1/responses", testStart)},
		})
		require.NoError(t, err)
		require.Equal(t, types.LLMFormatOpenAI, f.Job.LLMFormat)
	})

	t.Run("from llm request provider", func(t *testing.T) {
		f, err := New(JobParams{BeamID: testBeamID}, nil, map[string][]apievents.AuditEvent{
			"chunk-1": {
				requestEvent("r1", "https://llm.example.com/proxy", testStart),
				&apievents.AppSessionLLMRequest{
					Metadata: apievents.Metadata{Type: events.AppSessionLLMRequestSuccessEvent, Time: testStart},
					Provider: types.LLMProviderOpenAI,
				},
			},
		})
		require.NoError(t, err)
		require.Equal(t, types.LLMFormatOpenAI, f.Job.LLMFormat)
	})

	t.Run("caller's format wins", func(t *testing.T) {
		f, err := New(JobParams{BeamID: testBeamID, LLMFormat: types.LLMFormatOpenAI}, nil,
			map[string][]apievents.AuditEvent{
				"chunk-1": {requestEvent("r1", "https://api.anthropic.com/v1/messages", testStart)},
			})
		require.NoError(t, err)
		require.Equal(t, types.LLMFormatOpenAI, f.Job.LLMFormat)
	})

	t.Run("unrecognizable traffic leaves it unset", func(t *testing.T) {
		f, err := New(JobParams{BeamID: testBeamID}, nil, map[string][]apievents.AuditEvent{
			"chunk-1": {requestEvent("r1", "https://llm.example.com/proxy", testStart)},
		})
		require.NoError(t, err)
		require.Empty(t, f.Job.LLMFormat)
	})
}

// TestHTTPExchanges asserts bodies are reassembled in index order across chunk
// recordings, and that an exchange missing its response is reported incomplete
// rather than dropped.
func TestHTTPExchanges(t *testing.T) {
	exchanges, err := testFixture(t).HTTPExchanges()
	require.NoError(t, err)
	require.Len(t, exchanges, 2)

	// Ordered by request time.
	require.Equal(t, "r1", exchanges[0].RequestID)
	require.Equal(t, "r2", exchanges[1].RequestID)

	r1 := exchanges[0]
	require.Equal(t, testStart, r1.At)
	require.Equal(t, "POST", r1.Method)
	require.Equal(t, "https://api.anthropic.com/v1/messages", r1.URL)
	require.EqualValues(t, 200, r1.StatusCode)
	require.Equal(t, []byte("req-areq-b"), r1.RequestBody)
	// The response body's two chunks came from different recordings.
	require.Equal(t, []byte("resp-1resp-2"), r1.ResponseBody)
	require.True(t, r1.Complete)
	// Header names are lower-cased and the first value for a name wins.
	require.Equal(t, map[string]string{"x-codex-turn-metadata": "first"}, r1.Headers)

	r2 := exchanges[1]
	require.Equal(t, []byte("clipped"), r2.RequestBody)
	require.Empty(t, r2.ResponseBody)
	require.False(t, r2.Complete, "an exchange with no response envelope is incomplete")
	require.Zero(t, r2.StatusCode)
}

// TestHTTPExchangesGapInBody asserts a body whose chunks have a hole is still
// returned -- a truncated transcript beats none -- but is flagged.
func TestHTTPExchangesGapInBody(t *testing.T) {
	f, err := New(JobParams{BeamID: testBeamID}, nil, map[string][]apievents.AuditEvent{
		"chunk-1": {
			requestEvent("r1", "https://api.anthropic.com/v1/messages", testStart),
			requestChunk("r1", 0, "a", false),
			// chunk index 1 never made it into the export.
			requestChunk("r1", 2, "c", true),
			responseChunk("r1", 0, "resp", true),
			responseEvent("r1", 200, testStart),
		},
	})
	require.NoError(t, err)

	exchanges, err := f.HTTPExchanges()
	require.NoError(t, err)
	require.Len(t, exchanges, 1)
	require.Equal(t, []byte("ac"), exchanges[0].RequestBody)
	require.False(t, exchanges[0].Complete)
}

// TestSourceSearchEvents asserts the source applies the event-type and beam
// filters, pages, and orders -- the parts of a search request the replay sink
// depends on.
func TestSourceSearchEvents(t *testing.T) {
	src, err := testFixture(t).Source()
	require.NoError(t, err)
	ctx := context.Background()

	t.Run("filters by beam", func(t *testing.T) {
		found, next, err := src.SearchEvents(ctx, events.SearchEventsRequest{
			BeamID: testBeamID,
			Order:  types.EventOrderAscending,
		})
		require.NoError(t, err)
		require.Empty(t, next)
		require.Len(t, found, 2, "the third event belongs to another beam")
		for _, ev := range found {
			require.Equal(t, testBeamID, ev.(*apievents.AppSessionChunk).GetBeamID())
		}
	})

	t.Run("filters by event type", func(t *testing.T) {
		found, _, err := src.SearchEvents(ctx, events.SearchEventsRequest{
			BeamID:     testBeamID,
			EventTypes: []string{events.AppSessionHTTPRequestEvent},
		})
		require.NoError(t, err)
		require.Empty(t, found, "chunk recording events are not cluster events")
	})

	t.Run("ignores the time window", func(t *testing.T) {
		// Every consumer replays a fixture through a Job whose timestamps may be
		// zero or narrower than the export; the fixture is already scoped to one
		// beam, so the window is not applied.
		found, _, err := src.SearchEvents(ctx, events.SearchEventsRequest{BeamID: testBeamID})
		require.NoError(t, err)
		require.Len(t, found, 2)
	})

	t.Run("pages", func(t *testing.T) {
		var ids []string
		startKey := ""
		for range 10 {
			found, next, err := src.SearchEvents(ctx, events.SearchEventsRequest{
				BeamID:   testBeamID,
				Limit:    1,
				StartKey: startKey,
			})
			require.NoError(t, err)
			for _, ev := range found {
				ids = append(ids, ev.(*apievents.AppSessionChunk).SessionChunkID)
			}
			if next == "" {
				break
			}
			startKey = next
		}
		require.Equal(t, []string{"chunk-1", "chunk-2"}, ids)
	})

	t.Run("descending", func(t *testing.T) {
		found, _, err := src.SearchEvents(ctx, events.SearchEventsRequest{
			BeamID: testBeamID,
			Order:  types.EventOrderDescending,
		})
		require.NoError(t, err)
		require.Len(t, found, 2)
		require.Equal(t, "chunk-2", found[0].(*apievents.AppSessionChunk).SessionChunkID)
	})

	t.Run("rejects a bad start key", func(t *testing.T) {
		_, _, err := src.SearchEvents(ctx, events.SearchEventsRequest{StartKey: "not-a-key"})
		require.Error(t, err)
	})
}

// TestSourceStreamSessionEvents asserts the stream follows the audit log's
// contract: events then a closed channel on success, an error for a recording
// the export does not have.
func TestSourceStreamSessionEvents(t *testing.T) {
	src, err := testFixture(t).Source()
	require.NoError(t, err)

	t.Run("streams a recording and closes", func(t *testing.T) {
		evCh, errCh := src.StreamSessionEvents(context.Background(), session.ID("chunk-1"), 0)
		var got []apievents.AuditEvent
		for ev := range evCh {
			got = append(got, ev)
		}
		require.Len(t, got, 6)
		require.Empty(t, errCh, "a complete recording reports no error")
	})

	t.Run("honors the start index", func(t *testing.T) {
		evCh, _ := src.StreamSessionEvents(context.Background(), session.ID("chunk-1"), 4)
		var got []apievents.AuditEvent
		for ev := range evCh {
			got = append(got, ev)
		}
		require.Len(t, got, 2)
		require.Equal(t, "r2", got[0].(*apievents.AppSessionHTTPRequest).RequestId)
	})

	t.Run("start index past the end yields nothing", func(t *testing.T) {
		evCh, _ := src.StreamSessionEvents(context.Background(), session.ID("chunk-1"), 99)
		_, ok := <-evCh
		require.False(t, ok, "channel should close immediately")
	})

	t.Run("unknown recording errors", func(t *testing.T) {
		_, errCh := src.StreamSessionEvents(context.Background(), session.ID("nope"), 0)
		select {
		case err := <-errCh:
			require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for the missing recording error")
		}
	})

	t.Run("canceled context errors instead of looking complete", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, errCh := src.StreamSessionEvents(ctx, session.ID("chunk-1"), 0)
		select {
		case err := <-errCh:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for the cancellation error")
		}
	})
}
