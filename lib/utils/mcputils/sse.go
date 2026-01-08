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

package mcputils

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// ConnectSSEServer establishes an SSE stream with the MCP server and finds the
// endpoint used for posting client requests. If successful, the transport
// reader and message writer are returned.
func ConnectSSEServer(ctx context.Context, baseURL *url.URL, transport http.RoundTripper) (*SSEResponseReader, *SSERequestWriter, error) {
	httpClient := &http.Client{
		Transport: transport,
	}

	connectReq, err := http.NewRequestWithContext(ctx, "GET", baseURL.String(), nil)
	if err != nil {
		return nil, nil, trace.Wrap(err, "making SSE connection request")
	}
	connectReq.Header.Set("Accept", "text/event-stream")
	connectReq.Header.Set("Cache-Control", "no-cache")
	connectReq.Header.Set("Connection", "keep-alive")

	resp, err := httpClient.Do(connectReq)
	if err != nil {
		return nil, nil, trace.Wrap(err, "sending SSE request")
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, nil, trace.Errorf("unexpected status code: %d. Ensure the server URL is reachable, and is serving an MCP SSE server on the specified path.", resp.StatusCode)
	}

	reader := NewSSEResponseReader(resp.Body)
	endpointURL, err := reader.ReadEndpoint(ctx, baseURL)
	if err != nil {
		reader.Close()
		return nil, nil, trace.Wrap(err, "reading SSE server endpoint")
	}
	requestWriter := NewSSERequestWriter(httpClient, endpointURL)
	return reader, requestWriter, nil
}

// SSERequestWriter posts requests to the remote server. Implements
// MessageWriter.
type SSERequestWriter struct {
	httpClient  *http.Client
	endpointURL string
	sessionID   string
}

// NewSSERequestWriter creates a new SSERequestWriter.
func NewSSERequestWriter(httpClient *http.Client, endpointURL *url.URL) *SSERequestWriter {
	return &SSERequestWriter{
		httpClient:  httpClient,
		endpointURL: endpointURL.String(),
		sessionID:   endpointURL.Query().Get("sessionId"),
	}
}

// GetSessionID returns the MCP session ID tracked by the remote MCP server.
func (w *SSERequestWriter) GetSessionID() string {
	return w.sessionID
}

// GetEndpointURL returns the endpoint URL.
func (w *SSERequestWriter) GetEndpointURL() string {
	return w.endpointURL
}

// WriteMessage posts an HTTP request with the MCP message to the remote MCP
// server.
//
// Note that the HTTP response does not contain the MCP response. MCP response
// is sent through the SSE stream ¯\(ツ)/¯.
func (w *SSERequestWriter) WriteMessage(ctx context.Context, msg mcp.JSONRPCMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return trace.Wrap(err, "marshaling MCP request")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.endpointURL, bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err, "building SSE POST request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return trace.Wrap(err, "sending SSE request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return trace.BadParameter("SSE request returned %s", resp.Status)
	}
	return nil
}

// SSEResponseReader implements TransportReader that reads responses from the
// SSE stream with the MCP server.
type SSEResponseReader struct {
	io.Closer
	nextEvent func() (event, error, bool)
}

// NewSSEResponseReader creates a new SSEResponseReader. Input reader is usually the
// http body used for SSE stream.
func NewSSEResponseReader(reader io.ReadCloser) *SSEResponseReader {
	var mu sync.Mutex
	nextEvent, stopFunc := iter.Pull2(scanEvents(reader))
	return &SSEResponseReader{
		Closer: utils.CloseFunc(func() error {
			mu.Lock()
			stopFunc()
			mu.Unlock()
			return reader.Close()
		}),
		nextEvent: func() (event, error, bool) {
			mu.Lock()
			defer mu.Unlock()
			return nextEvent()
		},
	}
}

// ReadEndpoint reads the endpoint event and crafts the endpoint URL.
// This should be the first event after connecting to SSE server, and any error
// is critical.
func (r *SSEResponseReader) ReadEndpoint(ctx context.Context, baseURL *url.URL) (*url.URL, error) {
	evt, err, ok := r.nextEvent()
	if !ok {
		return nil, trace.Wrap(io.EOF, "reading SSE server message")
	} else if err != nil {
		return nil, trace.Wrap(err, "reading SSE server message")
	}
	if evt.name != sseEventEndpoint {
		return nil, trace.BadParameter("expecting endpoint event, got %s", evt.name)
	}
	endpointURI, err := baseURL.Parse(string(evt.data))
	if err != nil {
		return nil, trace.Wrap(err, "parsing endpoint data")
	}
	return endpointURI, nil
}

// ReadMessage reads the next SSE message event from SSE stream.
func (r *SSEResponseReader) ReadMessage(ctx context.Context) (string, error) {
	evt, err, ok := r.nextEvent()
	if !ok {
		return "", trace.Wrap(io.EOF, "reading SSE server message")
	} else if err != nil {
		return "", trace.Wrap(err, "reading SSE server message")
	}
	if evt.name != sseEventMessage {
		return "", newReaderParseError(trace.BadParameter("unexpected event type %s", evt.name))
	}
	return string(evt.data), nil
}

// Type returns "SSE".
func (r *SSEResponseReader) Type() string {
	return types.MCPTransportSSE
}

const (
	sseEventEndpoint string = "endpoint"
	sseEventMessage  string = "message"
)

// event is an event is a server-sent event.
// Copied from official go-sdk with minor modification (make all fields private):
// https://github.com/modelcontextprotocol/go-sdk/blob/a225d4dc7ded92f5492651a1bc60499b3be27044/mcp/event.go#L31
type event struct {
	name  string // the "event" field
	id    string // the "id" field
	data  []byte // the "data" field
	retry string // the "retry" field
}

// empty reports whether the Event is empty.
func (e event) empty() bool {
	return e.name == "" && e.id == "" && len(e.data) == 0 && e.retry == ""
}

func (e event) marshal() []byte {
	// Copied from official go-sdk with minor modification (no need for io.Writer):
	// https://github.com/modelcontextprotocol/go-sdk/blob/a225d4dc7ded92f5492651a1bc60499b3be27044/mcp/event.go#L44
	var b bytes.Buffer
	if e.name != "" {
		fmt.Fprintf(&b, "event: %s\n", e.name)
	}
	if e.id != "" {
		fmt.Fprintf(&b, "id: %s\n", e.id)
	}
	if e.retry != "" {
		fmt.Fprintf(&b, "retry: %s\n", e.retry)
	}
	fmt.Fprintf(&b, "data: %s\n\n", string(e.data))
	return b.Bytes()
}

// scanEvents iterates SSE events in the given scanner. The iterated error is
// terminal: if encountered, the stream is corrupt or broken and should no
// longer be used.
//
// Copied from official go-sdk with minor modification (use unexposed "event"):
// https://github.com/modelcontextprotocol/go-sdk/blob/a225d4dc7ded92f5492651a1bc60499b3be27044/mcp/event.go#L69
func scanEvents(r io.Reader) iter.Seq2[event, error] {
	scanner := bufio.NewScanner(r)
	const maxTokenSize = 1 * 1024 * 1024 // 1 MiB max line size
	scanner.Buffer(nil, maxTokenSize)

	// TODO: investigate proper behavior when events are out of order, or have
	// non-standard names.
	var (
		eventKey = []byte("event")
		idKey    = []byte("id")
		dataKey  = []byte("data")
		retryKey = []byte("retry")
	)

	return func(yield func(event, error) bool) {
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
			evt     event
			dataBuf *bytes.Buffer // if non-nil, preceding field was also data
		)
		flushData := func() {
			if dataBuf != nil {
				evt.data = dataBuf.Bytes()
				dataBuf = nil
			}
		}
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				flushData()
				// \n\n is the record delimiter
				if !evt.empty() && !yield(evt, nil) {
					return
				}
				evt = event{}
				continue
			}
			before, after, found := bytes.Cut(line, []byte{':'})
			if !found {
				yield(event{}, fmt.Errorf("malformed line in SSE stream: %q", string(line)))
				return
			}
			if !bytes.Equal(before, dataKey) {
				flushData()
			}
			switch {
			case bytes.Equal(before, eventKey):
				evt.name = strings.TrimSpace(string(after))
			case bytes.Equal(before, idKey):
				evt.id = strings.TrimSpace(string(after))
			case bytes.Equal(before, retryKey):
				evt.retry = strings.TrimSpace(string(after))
			case bytes.Equal(before, dataKey):
				data := bytes.TrimSpace(after)
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
				err = fmt.Errorf("event exceeded max line length of %d", maxTokenSize)
			}
			if !yield(event{}, err) {
				return
			}
		}
		flushData()
		if !evt.empty() {
			yield(evt, nil)
		}
	}
}
