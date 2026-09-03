// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Based on the official SDK implementation:
// https://github.com/modelcontextprotocol/go-sdk/blob/b4f957ff3c279051f9bcc88aa08e897add012a95/mcp/event.go

package mcputils

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strings"

	"github.com/gravitational/teleport"
)

// An Event is a server-sent event.
// See https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#fields.
type Event struct {
	Name  string // the "event" field
	ID    string // the "id" field
	Data  []byte // the "data" field
	Retry string // the "retry" field
}

// Empty reports whether the Event is empty.
func (e Event) Empty() bool {
	return e.Name == "" && e.ID == "" && len(e.Data) == 0 && e.Retry == ""
}

// writeEvent writes the event to w, and flushes.
func writeEvent(w io.Writer, evt Event) (int, error) {
	var b bytes.Buffer
	if evt.Name != "" {
		fmt.Fprintf(&b, "event: %s\n", evt.Name)
	}
	if evt.ID != "" {
		fmt.Fprintf(&b, "id: %s\n", evt.ID)
	}
	if evt.Retry != "" {
		fmt.Fprintf(&b, "retry: %s\n", evt.Retry)
	}
	fmt.Fprintf(&b, "data: %s\n\n", string(evt.Data))
	n, err := w.Write(b.Bytes())
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return n, err
}

// scanEvents iterates SSE events in the given scanner. The iterated error is
// terminal: if encountered, the stream is corrupt or broken and should no
// longer be used.
//
// TODO(rfindley): consider a different API here that makes failure modes more
// apparent.
func scanEvents(r io.Reader) iter.Seq2[Event, error] {
	scanner := bufio.NewScanner(r)
	// Tool discovery responses from servers exposing hundreds of tools can
	// exceed 1 MiB. Keep SSE lines bounded by the HTTP response size limit.
	scanner.Buffer(nil, teleport.MaxHTTPResponseSize)

	// TODO: investigate proper behavior when events are out of order, or have
	// non-standard names.
	var (
		eventKey = []byte("event")
		idKey    = []byte("id")
		dataKey  = []byte("data")
		retryKey = []byte("retry")
	)

	return func(yield func(Event, error) bool) {
		// iterate event from the wire.
		// https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#examples
		//
		//  - `key: value` line records.
		//  - Consecutive `data: ...` fields are joined with newlines.
		//  - Unrecognized fields are ignored. Since we only care about 'event', 'id', and
		//   'data', these are the only three we consider.
		//  - Lines starting with ":" are ignored.
		//  - Records are terminated with two consecutive newlines.
		var (
			evt     Event
			dataBuf *bytes.Buffer // if non-nil, preceding field was also data
			evtSize int
		)
		ensureRoom := func(addedSize int) bool {
			if addedSize > teleport.MaxHTTPResponseSize-evtSize {
				return false
			}
			evtSize += addedSize
			return true
		}
		flushData := func() {
			if dataBuf != nil {
				evt.Data = dataBuf.Bytes()
				dataBuf = nil
			}
		}
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				flushData()
				// \n\n is the record delimiter
				if !evt.Empty() && !yield(evt, nil) {
					return
				}
				evt = Event{}
				evtSize = 0
				continue
			}
			before, after, found := bytes.Cut(line, []byte{':'})
			if !found {
				yield(Event{}, fmt.Errorf("malformed line in SSE stream: %q", string(line)))
				return
			}
			if !bytes.Equal(before, dataKey) {
				flushData()
			}
			switch {
			case bytes.Equal(before, eventKey):
				value := strings.TrimSpace(string(after))
				if !ensureRoom(len(value)) {
					yield(Event{}, fmt.Errorf("event exceeded max size of %d", teleport.MaxHTTPResponseSize))
					return
				}
				evt.Name = value
			case bytes.Equal(before, idKey):
				value := strings.TrimSpace(string(after))
				if !ensureRoom(len(value)) {
					yield(Event{}, fmt.Errorf("event exceeded max size of %d", teleport.MaxHTTPResponseSize))
					return
				}
				evt.ID = value
			case bytes.Equal(before, retryKey):
				value := strings.TrimSpace(string(after))
				if !ensureRoom(len(value)) {
					yield(Event{}, fmt.Errorf("event exceeded max size of %d", teleport.MaxHTTPResponseSize))
					return
				}
				evt.Retry = value
			case bytes.Equal(before, dataKey):
				data := bytes.TrimSpace(after)
				addedSize := len(data)
				if dataBuf != nil {
					addedSize++ // Account for the inserted newline.
				}
				if !ensureRoom(addedSize) {
					yield(Event{}, fmt.Errorf("event exceeded max size of %d", teleport.MaxHTTPResponseSize))
					return
				}
				if dataBuf != nil {
					dataBuf.WriteByte('\n')
					dataBuf.Write(data)
				} else {
					dataBuf = new(bytes.Buffer)
					dataBuf.Write(data)
				}
			}
		}
		if err := scanner.Err(); err != nil {
			if errors.Is(err, bufio.ErrTooLong) {
				err = fmt.Errorf("event exceeded max line length of %d", teleport.MaxHTTPResponseSize)
			}
			if !yield(Event{}, err) {
				return
			}
		}
		flushData()
		if !evt.Empty() {
			yield(evt, nil)
		}
	}
}
