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
	"bytes"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
)

// HTTPExchange is one recorded LLM HTTP call, reassembled from the envelope and
// body-chunk events of the beam's chunk recordings.
type HTTPExchange struct {
	// RequestID identifies the exchange across the whole app session; it is what
	// the recorded request and response events are correlated by.
	RequestID string
	// At is the time the request was recorded. Exchanges are returned in this
	// order.
	At time.Time
	// Method and URL are the recorded request line.
	Method string
	URL    string
	// StatusCode is the recorded response status, or 0 when no response envelope
	// was recorded.
	StatusCode uint32
	// Headers are the recorded request headers, names lower-cased, first value
	// kept for a repeated name.
	Headers map[string]string
	// RequestBody and ResponseBody are the reassembled bodies. A response body is
	// the raw wire payload, so for a streamed completion it is the SSE stream.
	RequestBody  []byte
	ResponseBody []byte
	// Complete reports whether both bodies were reassembled contiguously through
	// their final chunk and a response envelope was recorded. It is false for an
	// exchange the export window clipped mid-flight, which is the usual reason a
	// fixture's last exchange looks truncated.
	Complete bool
}

// HTTPExchanges reassembles every LLM HTTP exchange in the fixture, ordered by
// request time.
//
// All chunk recordings are drained into one shared reassembly before any
// exchange is assembled: a request ID is unique across the app session rather
// than within one recording, so a single exchange's envelope and bodies can be
// split across recordings.
func (f *Fixture) HTTPExchanges() ([]HTTPExchange, error) {
	requests := map[string]*apievents.AppSessionHTTPRequest{}
	responses := map[string]*apievents.AppSessionHTTPResponse{}
	requestBodies := map[string]*bodyChunks{}
	responseBodies := map[string]*bodyChunks{}

	addChunk := func(bodies map[string]*bodyChunks, reqID string, index int64, data []byte, isLast bool) {
		b, ok := bodies[reqID]
		if !ok {
			b = &bodyChunks{}
			bodies[reqID] = b
		}
		b.add(index, data, isLast)
	}

	for id := range f.Chunks {
		evs, err := f.ChunkEvents(id)
		if err != nil {
			return nil, trace.Wrap(err, "decoding chunk recording %q", id)
		}
		for _, ev := range evs {
			switch e := ev.(type) {
			case *apievents.AppSessionHTTPRequest:
				requests[e.RequestId] = e
			case *apievents.AppSessionHTTPRequestBodyChunk:
				addChunk(requestBodies, e.RequestId, e.ChunkIndex, e.Data, e.IsLast)
			case *apievents.AppSessionHTTPResponse:
				responses[e.RequestId] = e
			case *apievents.AppSessionHTTPResponseBodyChunk:
				addChunk(responseBodies, e.RequestId, e.ChunkIndex, e.Data, e.IsLast)
			}
		}
	}

	exchanges := make([]HTTPExchange, 0, len(requests))
	for reqID, req := range requests {
		reqBody, reqComplete := requestBodies[reqID].assemble()
		respBody, respComplete := responseBodies[reqID].assemble()
		ex := HTTPExchange{
			RequestID:    reqID,
			At:           req.Metadata.Time,
			Method:       req.Method,
			URL:          req.Url,
			Headers:      headerMap(req.Headers),
			RequestBody:  reqBody,
			ResponseBody: respBody,
		}
		if resp := responses[reqID]; resp != nil {
			ex.StatusCode = resp.StatusCode
			ex.Complete = reqComplete && respComplete
		}
		exchanges = append(exchanges, ex)
	}
	// Stable so exchanges recorded within the same clock tick keep a fixed order
	// and a fixture replays identically every run.
	sort.SliceStable(exchanges, func(i, j int) bool {
		if exchanges[i].At.Equal(exchanges[j].At) {
			return exchanges[i].RequestID < exchanges[j].RequestID
		}
		return exchanges[i].At.Before(exchanges[j].At)
	})
	return exchanges, nil
}

// headerMap flattens recorded headers into lower-cased name/value pairs, keeping
// the first value seen for a repeated name.
func headerMap(headers []*apievents.HTTPHeader) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for _, h := range headers {
		name := strings.ToLower(h.Name)
		if _, ok := out[name]; !ok {
			out[name] = h.Value
		}
	}
	return out
}

// bodyChunks collects one body's recorded chunks. Chunks are sorted by index at
// assembly rather than assumed to arrive in order, because a body split across
// two chunk recordings is only in order if the recordings are read in order.
type bodyChunks struct {
	chunks  []bodyChunk
	sawLast bool
}

type bodyChunk struct {
	index int64
	data  []byte
}

func (b *bodyChunks) add(index int64, data []byte, isLast bool) {
	b.chunks = append(b.chunks, bodyChunk{index: index, data: data})
	b.sawLast = b.sawLast || isLast
}

// assemble concatenates the chunks in index order and reports whether the body
// is a faithful reassembly: every index from 0 up present exactly once, and the
// final chunk seen. A body with a gap is still returned -- a truncated
// transcript is more useful than none -- but is reported incomplete.
func (b *bodyChunks) assemble() ([]byte, bool) {
	if b == nil {
		// No body was recorded for this side of the exchange. An LLM request
		// always has both, but a fixture is also used to inspect partial exports,
		// so treat a missing body as empty and complete rather than as a defect.
		return nil, true
	}
	sort.SliceStable(b.chunks, func(i, j int) bool { return b.chunks[i].index < b.chunks[j].index })

	var buf bytes.Buffer
	next := int64(0)
	contiguous := true
	for _, c := range b.chunks {
		switch {
		case c.index == next:
			buf.Write(c.data)
			next++
		case c.index < next:
			// Duplicate chunk already written; ignore.
		default:
			// A chunk is missing, so what follows no longer joins onto what came
			// before. Keep the remaining data, flagged as an inexact reassembly.
			buf.Write(c.data)
			next = c.index + 1
			contiguous = false
		}
	}
	return buf.Bytes(), contiguous && b.sawLast
}
