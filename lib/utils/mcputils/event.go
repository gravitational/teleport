// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// All content in this file is copied from the official SDK without
// modifications:
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

const maxSSEEventLineSize = teleport.MaxHTTPResponseSize

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
	reader := bufio.NewReader(r)

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
		)
		yieldEvent := func() bool {
			if dataBuf != nil {
				evt.Data = dataBuf.Bytes()
				dataBuf = nil
			}
			if evt.Empty() {
				return true
			}
			if !yield(evt, nil) {
				return false
			}
			evt = Event{}
			return true
		}
		for {
			line, err := readSSEEventLine(reader)
			if err != nil && !errors.Is(err, io.EOF) {
				yield(Event{}, fmt.Errorf("error reading event: %w", err))
				return
			}
			line = bytes.TrimRight(line, "\r\n")
			isEOF := errors.Is(err, io.EOF)

			if len(line) == 0 {
				if !yieldEvent() {
					return
				}
				if isEOF {
					return
				}
				continue
			}
			before, after, found := bytes.Cut(line, []byte{':'})
			if !found {
				yield(Event{}, fmt.Errorf("malformed line in SSE stream: %q", string(line)))
				return
			}
			switch {
			case bytes.Equal(before, eventKey):
				evt.Name = strings.TrimSpace(string(after))
			case bytes.Equal(before, idKey):
				evt.ID = strings.TrimSpace(string(after))
			case bytes.Equal(before, retryKey):
				evt.Retry = strings.TrimSpace(string(after))
			case bytes.Equal(before, dataKey):
				data := bytes.TrimSpace(after)
				if dataBuf == nil {
					dataBuf = new(bytes.Buffer)
					dataBuf.Write(data)
				} else {
					dataBuf.WriteByte('\n')
					dataBuf.Write(data)
				}
			}

			if isEOF {
				yieldEvent()
				return
			}
		}
	}
}

func readSSEEventLine(reader *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		fragment, err := reader.ReadSlice('\n')
		if len(line)+len(fragment) > maxSSEEventLineSize {
			return nil, bufio.ErrTooLong
		}
		line = append(line, fragment...)
		if !errors.Is(err, bufio.ErrBufferFull) {
			return line, err
		}
	}
}
