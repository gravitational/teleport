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
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"
	"testing/synctest"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/itertools/stream"
)

func TestSSEEventsRead(t *testing.T) {
	for name, tc := range map[string]struct {
		input        string
		expectError  require.ErrorAssertionFunc
		expectEvents require.ValueAssertionFunc
	}{
		"data only": {
			input:       dataOnlyExample,
			expectError: require.NoError,
			expectEvents: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Len(tt, i1, 2, i2...)
				events := i1.([]SSEEvent)
				requireEqualEvent(tt, SSEEvent{Data: []byte("some text")}, events[0])
				requireEqualEvent(tt, SSEEvent{Data: []byte("another message\nwith two lines")}, events[1])
			},
		},
		"named events": {
			input:       namedEventsExample,
			expectError: require.NoError,
			expectEvents: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Len(tt, i1, 4, i2...)
				events := i1.([]SSEEvent)
				requireEqualEvent(tt, SSEEvent{Event: "userconnect", Data: []byte(`{"username": "bobby", "time": "02:33:48"}`)}, events[0])
				requireEqualEvent(tt, SSEEvent{Event: "usermessage", Data: []byte(`{"username": "bobby", "time": "02:34:11", "text": "Hi everyone."}`)}, events[1])
				requireEqualEvent(tt, SSEEvent{Event: "userdisconnect", Data: []byte(`{"username": "bobby", "time": "02:34:23"}`)}, events[2])
				requireEqualEvent(tt, SSEEvent{Event: "usermessage", Data: []byte(`{"username": "sean", "time": "02:34:36", "text": "Bye, bobby."}`)}, events[3])
			},
		},
		"mixed and matching events": {
			input:       mixingAndMatchingExample,
			expectError: require.NoError,
			expectEvents: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Len(tt, i1, 3, i2...)
				events := i1.([]SSEEvent)
				requireEqualEvent(tt, SSEEvent{Event: "userconnect", Data: []byte(`{"username": "bobby", "time": "02:33:48"}`)}, events[0])
				requireEqualEvent(tt, SSEEvent{Data: []byte("Here's a system message of some kind that will get used\nto accomplish some task.")}, events[1])
				requireEqualEvent(tt, SSEEvent{Event: "usermessage", Data: []byte(`{"username": "bobby", "time": "02:34:11", "text": "Hi everyone."}`)}, events[2])
			},
		},
		"field without colon": {
			input:       "id\n\nevent\ndata: something\n\nevent: another event",
			expectError: require.NoError,
			expectEvents: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Len(tt, i1, 3, i2...)
				events := i1.([]SSEEvent)
				// ID event with no values.
				requireEqualEvent(tt, SSEEvent{ID: ""}, events[0])
				requireEqualEvent(tt, SSEEvent{Event: "", Data: []byte("something")}, events[1])
				requireEqualEvent(tt, SSEEvent{Event: "another event"}, events[2])
			},
		},
		"multiple data fields": {
			input:       "event: hello\ndata: start of message\nid: 1\ndata: end of message\n",
			expectError: require.NoError,
			expectEvents: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Len(tt, i1, 1, i2...)
				events := i1.([]SSEEvent)
				requireEqualEvent(tt, SSEEvent{ID: "1", Event: "hello", Data: []byte("start of message\nend of message")}, events[0])
			},
		},
		"carriage return line endings": {
			input:       "event: hello\rdata: start of message\rdata: end of message\r",
			expectError: require.NoError,
			expectEvents: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Len(tt, i1, 1, i2...)
				events := i1.([]SSEEvent)
				requireEqualEvent(tt, SSEEvent{Event: "hello", Data: []byte("start of message\nend of message")}, events[0])
			},
		},
		"carriage return and line feed line endings": {
			input:       "event: hello\r\ndata: start of message\r\ndata: end of message\r\n",
			expectError: require.NoError,
			expectEvents: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Len(tt, i1, 1, i2...)
				events := i1.([]SSEEvent)
				requireEqualEvent(tt, SSEEvent{Event: "hello", Data: []byte("start of message\nend of message")}, events[0])
			},
		},
		"preserve data whitespace": {
			input:       "data:  token  \ndata:\ttab\t\n\n",
			expectError: require.NoError,
			expectEvents: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Len(tt, i1, 1, i2...)
				events := i1.([]SSEEvent)
				requireEqualEvent(tt, SSEEvent{Data: []byte(" token  \n\ttab\t")}, events[0])
			},
		},
		"comment between fields": {
			input:       "event: message\n: ping\ndata: body\n\n",
			expectError: require.NoError,
			expectEvents: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Len(tt, i1, 1, i2...)
				events := i1.([]SSEEvent)
				requireEqualEvent(tt, SSEEvent{Event: "message", Data: []byte("body")}, events[0])
			},
		},
		"leading empty data line": {
			input:       "data:\ndata: foo\n\n",
			expectError: require.NoError,
			expectEvents: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Len(tt, i1, 1, i2...)
				events := i1.([]SSEEvent)
				requireEqualEvent(tt, SSEEvent{Data: []byte("\nfoo")}, events[0])
			},
		},
		"multiple empty data lines": {
			input:       "data:\ndata:\ndata: foo\n\n",
			expectError: require.NoError,
			expectEvents: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Len(tt, i1, 1, i2...)
				events := i1.([]SSEEvent)
				requireEqualEvent(tt, SSEEvent{Data: []byte("\n\nfoo")}, events[0])
			},
		},
		"single event data exceeds max size": {
			input: "data: " + strings.Repeat("x", MaxSSEReadEventSize),
			expectError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(tt, err, ErrSSEEventTooLarge, i...)
			},
			expectEvents: require.Empty,
		},
		"single event field exceeds max size": {
			input: "event: " + strings.Repeat("x", MaxSSEReadEventSize),
			expectError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(tt, err, ErrSSEEventTooLarge, i...)
			},
			expectEvents: require.Empty,
		},
		"single event exceeds max size multi lines": {
			input: "data: " + strings.Repeat("x", MaxSSEReadEventSize/2) + "\ndata:" + strings.Repeat("y", MaxSSEReadEventSize/2),
			expectError: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(tt, err, ErrSSEEventTooLarge, i...)
			},
			expectEvents: require.Empty,
		},
		"invalid event field is ignored": {
			input:       "event: hello\nrandom: error\ndata: something",
			expectError: require.NoError,
			expectEvents: func(tt require.TestingT, i1 any, i2 ...any) {
				require.Len(tt, i1, 1, i2...)
				events := i1.([]SSEEvent)
				requireEqualEvent(tt, SSEEvent{Event: "hello", Data: []byte("something")}, events[0])
			},
		},
		"empty": {
			input:        "",
			expectError:  require.NoError,
			expectEvents: require.Empty,
		},
	} {
		t.Run(name, func(t *testing.T) {
			events, err := stream.Collect(ReadSSEEvents(strings.NewReader(tc.input)))
			tc.expectError(t, err)
			tc.expectEvents(t, events)
		})
	}
}

func TestSSEEventsReadBareCRLiveStream(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		chunks := make(chan []byte, 1)
		reader := &chunkReader{chunks: chunks}

		events := make(chan SSEEvent, 1)
		errs := make(chan error, 1)
		go func() {
			for event, err := range ReadSSEEvents(reader) {
				if err != nil {
					errs <- err
					return
				}
				events <- event
				return
			}
		}()

		chunks <- []byte("data: ok\r\r")
		synctest.Wait()

		select {
		case event := <-events:
			requireEqualEvent(t, SSEEvent{Data: []byte("ok")}, event)
		case err := <-errs:
			t.Fatalf("expected no errors but got %v", err)
		}
	})
}

func TestSSEEventsReadSplitCRLFLiveStream(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		chunks := make(chan []byte, 1)
		reader := &chunkReader{chunks: chunks}

		events := make(chan SSEEvent, 1)
		errs := make(chan error, 1)
		go func() {
			for event, err := range ReadSSEEvents(reader) {
				if err != nil {
					errs <- err
					return
				}
				events <- event
				return
			}
		}()

		chunks <- []byte("data: ok\r")
		synctest.Wait()
		requireNoSSEEvent(t, events, errs)

		chunks <- []byte("\n")
		synctest.Wait()
		requireNoSSEEvent(t, events, errs)

		chunks <- []byte("data: there\r\n\r\n")
		synctest.Wait()

		select {
		case event := <-events:
			requireEqualEvent(t, SSEEvent{Data: []byte("ok\nthere")}, event)
		case err := <-errs:
			t.Fatalf("expected no errors but got %v", err)
		}
	})
}

func TestSSEEventsWrite(t *testing.T) {
	expectedString := func(str string) require.ValueAssertionFunc {
		return func(tt require.TestingT, i1 any, i2 ...any) {
			require.IsType(t, []byte{}, i1, "expected result to be `[]byte` but got %T", i1)
			b := i1.([]byte)
			require.Equal(tt, str, string(b), i2...)
		}
	}

	for name, tc := range map[string]struct {
		event        SSEEvent
		expectError  require.ErrorAssertionFunc
		expectOutput require.ValueAssertionFunc
	}{
		"no data event": {
			event:        SSEEvent{ID: "1", Event: "hello"},
			expectError:  require.NoError,
			expectOutput: expectedString("event: hello\nid: 1\n\n"),
		},
		"data only": {
			event:        SSEEvent{Data: []byte("some text")},
			expectError:  require.NoError,
			expectOutput: expectedString("data: some text\n\n"),
		},
		"multiline data": {
			event:        SSEEvent{Data: []byte("another message\nwith two lines")},
			expectError:  require.NoError,
			expectOutput: expectedString("data: another message\ndata: with two lines\n\n"),
		},
		"multiline data escaped": {
			event:        SSEEvent{Data: []byte("{\"response\": \"hello\\nworld\"}\n{\"response\": \"second\"}")},
			expectError:  require.NoError,
			expectOutput: expectedString("data: {\"response\": \"hello\\nworld\"}\ndata: {\"response\": \"second\"}\n\n"),
		},
		"carriage return data": {
			event:        SSEEvent{Data: []byte("ok\revent: injected")},
			expectError:  require.NoError,
			expectOutput: expectedString("data: ok\ndata: event: injected\n\n"),
		},
		"carriage return newline data": {
			event:        SSEEvent{Data: []byte("first\r\nsecond")},
			expectError:  require.NoError,
			expectOutput: expectedString("data: first\ndata: second\n\n"),
		},
		"named events": {
			event:        SSEEvent{Event: "userconnect", Data: []byte(`{"username": "bobby", "time": "02:33:48"}`)},
			expectError:  require.NoError,
			expectOutput: expectedString("event: userconnect\ndata: {\"username\": \"bobby\", \"time\": \"02:33:48\"}\n\n"),
		},
		"all fields": {
			event:        SSEEvent{ID: "1", Event: "hello", Data: []byte("start of message\nend of message"), Retry: "5"},
			expectError:  require.NoError,
			expectOutput: expectedString("event: hello\nid: 1\ndata: start of message\ndata: end of message\nretry: 5\n\n"),
		},
		"invalid fields": {
			event: SSEEvent{ID: "1\n\r", Event: "hello", Data: []byte("start of message\nend of message"), Retry: "5"},
			expectError: func(tt require.TestingT, err error, i ...any) {
				require.True(tt, trace.IsBadParameter(err), "expected error %v to be BadParameter", err)
			},
			expectOutput: require.Empty,
		},
		"invalid retry field": {
			event: SSEEvent{Event: "hello", Data: []byte("start of message\nend of message"), Retry: "non-digits"},
			expectError: func(tt require.TestingT, err error, i ...any) {
				require.True(tt, trace.IsBadParameter(err), "expected error %v to be BadParameter", err)
			},
			expectOutput: require.Empty,
		},
		"empty": {
			event:        SSEEvent{},
			expectError:  require.NoError,
			expectOutput: require.Empty,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			_, err := WriteSSEEvent(&buf, tc.event)
			tc.expectError(t, err)
			tc.expectOutput(t, buf.Bytes())
		})
	}
}

// TestSSEEventsWriteRead ensures both write and read SSE events functions
// produce results that are compatible with each other.
func TestSSEEventsWriteRead(t *testing.T) {
	for name, input := range map[string]string{
		// We use the version that doesn't include the comment as it is not read
		// by the reader.
		"data only":                 dataOnlyWithoutCommentExample,
		"named events":              namedEventsExample,
		"mixed and matching events": mixingAndMatchingExample,
		"multiple data fields":      "event: hello\nid: 1\ndata: start of message\ndata: end of message\n\n",
		"empty":                     "",
	} {
		t.Run(name, func(t *testing.T) {
			inputEvents, err := stream.Collect(ReadSSEEvents(strings.NewReader(input)))
			require.NoError(t, err)

			var buf bytes.Buffer
			for _, event := range inputEvents {
				_, err := WriteSSEEvent(&buf, event)
				require.NoError(t, err)
			}

			require.Equal(t, input, buf.String())
		})
	}
}

// TestSSEEventsReadFieldsAreCopied ensures ReadSSEEvents returns events whose
// fields own their bytes rather than aliasing the scanner's internal buffer.
// bufio.Scanner compacts its buffer between Scan() calls, so any retained
// sub-slice of scanner.Bytes() would be overwritten by later reads.
func TestSSEEventsReadFieldsAreCopied(t *testing.T) {
	const n = 200

	var buf bytes.Buffer
	for i := range n {
		_, err := WriteSSEEvent(&buf, SSEEvent{
			ID:    fmt.Sprintf("%d", i),
			Event: fmt.Sprintf("e%d", i),
			// Multi-line data exercises the append branch of Data
			// accumulation in addition to the initial allocation.
			Data: fmt.Appendf(nil, "first-%d\nsecond-%d", i, i),
		})
		require.NoError(t, err)
	}

	// We force multiple Scan calls by using a one byte reader.
	got, err := stream.Collect(ReadSSEEvents(iotest.OneByteReader(&buf)))
	require.NoError(t, err)
	require.Len(t, got, n)

	for i, ev := range got {
		require.Equal(t, fmt.Sprintf("%d", i), ev.ID)
		require.Equal(t, fmt.Sprintf("e%d", i), ev.Event)
		require.Equal(t, fmt.Appendf(nil, "first-%d\nsecond-%d", i, i), ev.Data)
	}
}

func FuzzReadSSEEvents(f *testing.F) {
	f.Add("")
	f.Add(dataOnlyExample)
	f.Add(namedEventsExample)
	f.Add(mixingAndMatchingExample)
	f.Fuzz(func(t *testing.T, a string) {
		reader := strings.NewReader(a)
		require.NotPanics(t, func() {
			for range ReadSSEEvents(reader) {
			}
		})
	})
}

func requireEqualEvent(t require.TestingT, expected SSEEvent, target SSEEvent) {
	require.Equal(t, expected.Event, target.Event)
	require.Equal(t, expected.Data, target.Data)
	require.Equal(t, expected.ID, target.ID)
	require.Equal(t, expected.Retry, target.Retry)
}

func requireNoSSEEvent(t *testing.T, events <-chan SSEEvent, errs <-chan error) {
	t.Helper()

	select {
	case event := <-events:
		require.Failf(t, "unexpected SSE event", "event = %+v", event)
	case err := <-errs:
		t.Fatalf("expected no errors but got %v", err)
	default:
	}
}

type chunkReader struct {
	chunks  <-chan []byte
	pending []byte
}

func (r *chunkReader) Read(p []byte) (int, error) {
	for len(r.pending) == 0 {
		chunk, ok := <-r.chunks
		if !ok {
			return 0, io.EOF
		}
		r.pending = chunk
	}

	n := copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}

// SSE stream examples from https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#examples
//
// Note all examples here contain two line breaks at the end to simulate the end
// of message.
const (
	dataOnlyExample = `: this is a test stream

data: some text

data: another message
data: with two lines

`
	namedEventsExample = `event: userconnect
data: {"username": "bobby", "time": "02:33:48"}

event: usermessage
data: {"username": "bobby", "time": "02:34:11", "text": "Hi everyone."}

event: userdisconnect
data: {"username": "bobby", "time": "02:34:23"}

event: usermessage
data: {"username": "sean", "time": "02:34:36", "text": "Bye, bobby."}

`
	mixingAndMatchingExample = `event: userconnect
data: {"username": "bobby", "time": "02:33:48"}

data: Here's a system message of some kind that will get used
data: to accomplish some task.

event: usermessage
data: {"username": "bobby", "time": "02:34:11", "text": "Hi everyone."}

`
	dataOnlyWithoutCommentExample = `data: some text

data: another message
data: with two lines

`
)
