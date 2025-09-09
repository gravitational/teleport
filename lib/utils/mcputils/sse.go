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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/gravitational/teleport/api/types"
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
	scanner *bufio.Scanner
}

// NewSSEResponseReader creates a new SSEResponseReader. Input reader is usually the
// http body used for SSE stream.
func NewSSEResponseReader(reader io.ReadCloser) *SSEResponseReader {
	return &SSEResponseReader{
		Closer:  reader,
		scanner: bufio.NewScanner(reader),
	}
}

// ReadEndpoint reads the endpoint event and crafts the endpoint URL.
// This should be the first event after connecting to SSE server, and any error
// is critical.
func (r *SSEResponseReader) ReadEndpoint(ctx context.Context, baseURL *url.URL) (*url.URL, error) {
	evt, err := r.nextEvent()
	if err != nil {
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
	evt, err := r.nextEvent()
	if err != nil {
		return "", trace.Wrap(err)
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
type event struct {
	name string
	data []byte
}

// nextEvent reads one sse event from the wire.
//
// Logic is copied from golang internal mcp lib which might get released
// officially someday:
// https://cs.opensource.google/go/x/tools/+/refs/tags/v0.34.0:internal/mcp/sse.go.
//
// Original comment from above go source:
// https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events#examples
//   - `key: value` line records.
//   - Consecutive `data: ...` fields are joined with newlines.
//   - Unrecognized fields are ignored. Since we only care about 'event' and
//     'data', these are the only two we consider.
//   - Lines starting with ":" are ignored.
//   - Records are terminated with two consecutive newlines.
func (r *SSEResponseReader) nextEvent() (event, error) {
	var (
		evt         event
		lastWasData bool // if set, preceding data field was also data
	)
	for r.scanner.Scan() {
		line := r.scanner.Bytes()
		if len(line) == 0 && (evt.name != "" || len(evt.data) > 0) {
			return evt, nil
		}
		before, after, found := bytes.Cut(line, []byte{':'})
		if !found {
			return evt, fmt.Errorf("malformed line in SSE stream: %q", string(line))
		}
		switch {
		case bytes.Equal(before, []byte("event")):
			evt.name = strings.TrimSpace(string(after))
		case bytes.Equal(before, []byte("data")):
			data := bytes.TrimSpace(after)
			if lastWasData {
				evt.data = slices.Concat(evt.data, []byte{'\n'}, data)
			} else {
				evt.data = data
			}
			lastWasData = true
		}
	}
	return evt, io.EOF
}
