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

package httplib

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"iter"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
)

var (
	// ErrSSEEventTooLarge is the error returned when SSE event is larger than
	// [MaxSSEReadEventSize].
	ErrSSEEventTooLarge = errors.New("sse event exceeded max size")
)

// MaxSSEReadEventSize defines the max size that can be read from an [io.Reader]
// to complete the event.
const MaxSSEReadEventSize = teleport.MaxHTTPResponseSize

// SSEEvent is a server-sent event.
type SSEEvent struct {
	Event string
	ID    string
	Data  []byte
	Retry string
}

// Empty reports whether the event is empty.
func (e SSEEvent) Empty() bool {
	return e.len() == 0
}

// Equal returns `true` if both events have the same contents.
func (e SSEEvent) Equal(b SSEEvent) bool {
	return e.Event == b.Event && e.ID == b.ID &&
		bytes.Equal(e.Data, b.Data) && e.Retry == b.Retry
}

func (e SSEEvent) len() int {
	return len(e.Event) + len(e.ID) + len(e.Data) + len(e.Retry)
}

// Validates the fields of the SSE event.
func (e SSEEvent) validate() error {
	switch {
	case strings.ContainsAny(e.Event, "\r\n"):
		return trace.BadParameter("Event field cannot contain line feed or carriage return")
	case strings.ContainsAny(e.ID, "\r\n"):
		return trace.BadParameter("ID field cannot contain line feed or carriage return")
	case strings.ContainsAny(e.Retry, "\r\n"):
		return trace.BadParameter("Retry field cannot contain line feed or carriage return")
	case !onlyDigits(e.Retry):
		return trace.BadParameter("Retry field can only contain digits")
	}

	return nil
}

// ReadSSEEvents reads SSE events from the provided reader.
func ReadSSEEvents(r io.Reader) iter.Seq2[SSEEvent, error] {
	return func(yield func(SSEEvent, error) bool) {
		var currentEvent *SSEEvent
		scanner := bufio.NewScanner(r)
		scanner.Buffer(nil, MaxSSEReadEventSize)
		scanner.Split(scanSSELines())

		yieldEvent := func() bool {
			// This handles cases where the stream has empty lines, so we
			// consumed them, and there is no need to invoke `yield`.
			if currentEvent == nil {
				return true
			}
			if !yield(*currentEvent, nil) {
				return false
			}

			// Reset for next event.
			currentEvent = nil
			return true
		}

		// For fields that get their value replaced, we compare against the
		// current total of all fields values rather than per-field, so
		// re-assigned fields still count their prior length.
		//
		// This can reject some valid events but reliably bounds memory.
		ensureRoom := func(addedSize int) bool {
			if currentEvent == nil {
				currentEvent = &SSEEvent{}
			}
			if currentEvent.len()+addedSize > MaxSSEReadEventSize {
				return false
			}
			return true
		}

		for scanner.Scan() {
			line := scanner.Bytes()

			if len(line) == 0 {
				if !yieldEvent() {
					return
				}
				continue
			}
			// Ignore "comment" lines.
			//
			// > If the line starts with a U+003A COLON character (:)
			// >   Ignore the line.
			//
			// https://html.spec.whatwg.org/multipage/server-sent-events.html#event-stream-interpretation
			if line[0] == ':' {
				continue
			}

			// A field line without colon is considered valid with empty value.
			//
			// > Note: If a line doesn't contain a colon, the entire line is treated as the field name with an empty value string.
			//
			// From https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#fields
			before, after, _ := bytes.Cut(line, []byte{':'})
			// Per spec we must only trim the leading space from the field data.
			// This avoids breaking the data sent by the server.
			after = bytes.TrimPrefix(after, []byte{' '})
			switch {
			case bytes.Equal(before, sseEventKey):
				if !ensureRoom(len(after)) {
					yield(SSEEvent{}, trace.Wrap(ErrSSEEventTooLarge))
					return
				}
				currentEvent.Event = string(after)
			case bytes.Equal(before, sseIDKey):
				if !ensureRoom(len(after)) {
					yield(SSEEvent{}, trace.Wrap(ErrSSEEventTooLarge))
					return
				}
				currentEvent.ID = string(after)
			case bytes.Equal(before, sseRetryKey):
				if !ensureRoom(len(after)) {
					yield(SSEEvent{}, trace.Wrap(ErrSSEEventTooLarge))
					return
				}
				currentEvent.Retry = string(after)
			case bytes.Equal(before, sseDataKey):
				// Account for the new line added after the `data` field.
				if !ensureRoom(len(after) + 1) {
					yield(SSEEvent{}, trace.Wrap(ErrSSEEventTooLarge))
					return
				}
				// Cases where first data is empty, we cannot rely only on
				// append as it would still return nil (so we wouldn't know a
				// data field happened before). That's why we initialize the
				// slice here.
				if currentEvent.Data == nil {
					currentEvent.Data = make([]byte, 0, len(after))
				} else {
					currentEvent.Data = append(currentEvent.Data, '\n')
				}
				currentEvent.Data = append(currentEvent.Data, after...)
			default:
				// Following the spec, unknown fields are just ignored.
				continue
			}
		}

		if err := scanner.Err(); err != nil {
			if errors.Is(err, bufio.ErrTooLong) {
				yield(SSEEvent{}, trace.Wrap(ErrSSEEventTooLarge))
				return
			}

			yield(SSEEvent{}, trace.Wrap(err))
			return
		}

		yieldEvent()
	}
}

// WriteSSEEvent writes an (non-empty) SSE event into the provided Writer.
//
// This function does not flush the writer. Callers using HTTP streaming should
// flush their response after a successful write when events must be delivered
// promptly.
func WriteSSEEvent(w io.Writer, event SSEEvent) (int, error) {
	// Dropping empty events is acceptable as they don't carry values (only
	// fields).
	//
	// In case callers need to write empty fields, we'd need a separate writer
	// that preserves empty fields.
	if event.Empty() {
		return 0, nil
	}
	if err := event.validate(); err != nil {
		return 0, trace.Wrap(err)
	}

	// Use internal buffer so we only write once to the target writer.
	var buf bytes.Buffer
	if event.Event != "" {
		writeField(&buf, sseEventKey, event.Event)
	}
	if event.ID != "" {
		writeField(&buf, sseIDKey, event.ID)
	}
	for data := range splitSSEDataLines(event.Data) {
		buf.Write(sseDataKey)
		buf.WriteString(": ")
		buf.Write(data)
		buf.WriteByte('\n')
	}
	if event.Retry != "" {
		writeField(&buf, sseRetryKey, event.Retry)
	}
	buf.WriteByte('\n')

	n, err := buf.WriteTo(w)
	return int(n), trace.Wrap(err)
}

// writeField internal helper that writes fields to buffer without allocating
// additional strings.
func writeField(b *bytes.Buffer, fieldName []byte, fieldData string) {
	b.Write(fieldName)
	b.WriteString(": ")
	b.WriteString(fieldData)
	b.WriteByte('\n')
}

// scanSSELines implements a custom scan line for bytes.Scanner that honors the
// SSE spec in addition to the [MaxSSEReadEventSize].
//
// By the spec, the data can be split using \r (cr) \n (lf) following the end of
// line definition:
//
//	end-of-line   = ( cr lf / cr / lf )
//
// https://html.spec.whatwg.org/multipage/server-sent-events.html#parsing-an-event-stream
func scanSSELines() bufio.SplitFunc {
	var skipLF bool

	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		// If previous chunk ended on \r, swallow a leading \n here so \r\n
		// count as one terminator across reads. Bare \r line endings still work.
		if skipLF {
			if len(data) == 0 {
				return 0, nil, nil
			}
			skipLF = false
			if data[0] == '\n' {
				return 1, nil, nil
			}
		}

		for i, b := range data {
			if i > MaxSSEReadEventSize {
				return 0, nil, bufio.ErrTooLong
			}

			switch b {
			case '\n':
				return i + 1, data[:i], nil
			case '\r':
				// \r\n must count a single line break as per spec.
				if i+1 < len(data) && data[i+1] == '\n' {
					return i + 2, data[:i], nil
				}
				if i+1 == len(data) && !atEOF {
					skipLF = true
				}
				return i + 1, data[:i], nil
			}
		}

		if len(data) > MaxSSEReadEventSize {
			return 0, nil, bufio.ErrTooLong
		}

		if atEOF && len(data) > 0 {
			return len(data), data, nil
		}

		return 0, nil, nil
	}
}

// splitSSEDataLines splits data into multiple event lines.
//
// By the spec, the data can be split using \r (cr) \n (lf) following the end of
// line definition:
//
//	end-of-line   = ( cr lf / cr / lf )
//
// https://html.spec.whatwg.org/multipage/server-sent-events.html#parsing-an-event-stream
func splitSSEDataLines(data []byte) iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		// No data available, return nothing to the caller.
		if len(data) == 0 {
			return
		}

		for {
			i := bytes.IndexAny(data, "\r\n")
			if i < 0 {
				yield(data)
				return
			}

			if !yield(data[:i]) {
				return
			}

			// \r\n must count a single line break as per spec.
			if data[i] == '\r' && i+1 < len(data) && data[i+1] == '\n' {
				data = data[i+2:]
			} else {
				data = data[i+1:]
			}
		}
	}
}

func onlyDigits(str string) bool {
	return strings.IndexFunc(str, func(r rune) bool {
		return r < '0' || r > '9'
	}) == -1
}

// SSE supported fields names.
//
// Ref: https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#fields.
var (
	sseEventKey = []byte("event")
	sseIDKey    = []byte("id")
	sseDataKey  = []byte("data")
	sseRetryKey = []byte("retry")
)
