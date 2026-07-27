// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package testing

import (
	"io"
	"net/http"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/httplib/sse"
	"github.com/gravitational/teleport/lib/itertools/stream"
)

// DiscardResponseWriter is a minimal [http.ResponseWriter] + [http.Flusher]
// that discards all writes.
type DiscardResponseWriter struct {
	header http.Header
}

// NewDiscardResponseWriter creates a [DiscardResponseWriter] with the given
// Content-Type header set (when non-empty).
func NewDiscardResponseWriter(contentType string) *DiscardResponseWriter {
	h := http.Header{}
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	return &DiscardResponseWriter{header: h}
}

func (w *DiscardResponseWriter) Header() http.Header         { return w.header }
func (w *DiscardResponseWriter) Write(p []byte) (int, error) { return len(p), nil }
func (w *DiscardResponseWriter) WriteHeader(int)             {}
func (w *DiscardResponseWriter) Flush()                      {}

// FailingResponseWriter is a minimal [http.ResponseWriter] + [http.Flusher]
// whose Write always fails, simulating a broken downstream connection.
type FailingResponseWriter struct {
	header http.Header
}

// NewFailingResponseWriter creates a [FailingResponseWriter] with the given
// Content-Type header set (when non-empty).
func NewFailingResponseWriter(contentType string) *FailingResponseWriter {
	h := http.Header{}
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	return &FailingResponseWriter{header: h}
}

func (w *FailingResponseWriter) Header() http.Header       { return w.header }
func (w *FailingResponseWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func (w *FailingResponseWriter) WriteHeader(int)           {}
func (w *FailingResponseWriter) Flush()                    {}

// ReadSSEOneEvent parses str and asserts it contains exactly one SSE event,
// which it returns.
func ReadSSEOneEvent(str string) (sse.Event, error) {
	events, err := stream.Collect(sse.ReadEvents(strings.NewReader(str)))
	if err != nil {
		return sse.Event{}, trace.Wrap(err)
	}
	if len(events) != 1 {
		return sse.Event{}, trace.BadParameter("must contain exactly one SSE event")
	}
	return events[0], nil
}
