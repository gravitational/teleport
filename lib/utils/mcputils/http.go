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
	"bytes"
	"context"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"

	"github.com/gravitational/trace"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// ServerMessageProcessor defines an interface that process JSON RPC responses
// and notifications.
type ServerMessageProcessor interface {
	// ProcessResponse process a response and returns the message for client.
	ProcessResponse(context.Context, *jsonrpc.Response) jsonrpc.Message
	// ProcessNotification process a notification and returns the message for client.
	ProcessNotification(context.Context, *jsonrpc.Request) jsonrpc.Message
}

// ReplaceHTTPResponse handles replacing the MCP server response for the
// streamable HTTP transport.
//
// https://modelcontextprotocol.io/specification/2025-06-18/basic/transports#streamable-http
func ReplaceHTTPResponse(ctx context.Context, resp *http.Response, processor ServerMessageProcessor) error {
	// Nothing to replace.
	if resp.StatusCode != http.StatusOK || resp.ContentLength == 0 {
		return nil
	}

	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return trace.Wrap(err)
	}
	switch mediaType {
	case "application/json":
		// Single response.
		respBody, err := utils.ReadAtMost(resp.Body, teleport.MaxHTTPRequestSize)
		if err != nil {
			return trace.Wrap(err)
		}
		respFromServer, err := unmarshalResponse(string(respBody))
		if err != nil {
			return trace.Wrap(err)
		}
		respToClient := processor.ProcessResponse(ctx, respFromServer)
		respToClientAsBody, err := jsonrpc.EncodeMessage(respToClient)
		if err != nil {
			return trace.Wrap(err)
		}
		resp.Body = io.NopCloser(bytes.NewReader(respToClientAsBody))

		// Make sure content length in both the response field and the header
		// are updated.
		resp.ContentLength = int64(len(respToClientAsBody))
		resp.Header.Set("Content-Length", strconv.FormatInt(int64(len(respToClientAsBody)), 10))
		return nil

	case "text/event-stream":
		// Multiple messages (response or notification) can be sent through SSE.
		// Instead of reading all messages then replacing them, here we replace
		// the body with a reader that process the event one a time.
		resp.Body = &httpSSEResponseReplacer{
			ctx:               ctx,
			SSEResponseReader: NewSSEResponseReader(resp.Body),
			processor:         processor,
		}

		// Content-Length should be -1 from server for streams. Force to -1 again just to
		// be sure.
		resp.ContentLength = -1
		resp.Header.Del("Content-Length")
		return nil
	default:
		return trace.BadParameter("unsupported response type %s", mediaType)
	}
}

type httpSSEResponseReplacer struct {
	*SSEResponseReader
	ctx       context.Context
	processor ServerMessageProcessor
	buf       []byte
}

func (r *httpSSEResponseReplacer) Read(p []byte) (int, error) {
	if len(r.buf) != 0 {
		n := copy(p, r.buf)
		r.buf = r.buf[n:]
		return n, nil
	}

	msg, err := r.ReadMessage(r.ctx)
	if err != nil {
		if utils.IsOKNetworkError(err) {
			return 0, io.EOF
		}
		return 0, trace.Wrap(err)
	}

	base, err := jsonrpc.DecodeMessage([]byte(msg))
	if err != nil {
		return 0, trace.Wrap(err)
	}

	var respToClient jsonrpc.Message
	switch v := base.(type) {
	case *jsonrpc.Response:
		respToClient = r.processor.ProcessResponse(r.ctx, v)
	case *jsonrpc.Request:
		if v.ID.IsValid() {
			return 0, trace.BadParameter("message is not a response or a notification")
		}
		respToClient = r.processor.ProcessNotification(r.ctx, v)
	default:
		return 0, trace.BadParameter("message is not a response or a notification")
	}

	respToSendAsBody, err := jsonrpc.EncodeMessage(respToClient)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	// Convert to SSE.
	e := event{
		name: sseEventMessage,
		data: respToSendAsBody,
	}
	r.buf = e.marshal()
	return r.Read(p)
}

// HTTPReaderWriter implements MessageWriter and TransportReader for
// streamable HTTP transport.
type HTTPReaderWriter struct {
	targetConn     mcp.Connection
	messagesToRead chan jsonrpc.Message
}

// NewHTTPReaderWriter creates a new HTTPReaderWriter that implements
// MessageWriter and TransportReader that connects to provided serverURL in
// streamable HTTP transport.
func NewHTTPReaderWriter(
	ctx context.Context,
	serverURL string,
	httpClient *http.Client,
) (*HTTPReaderWriter, error) {
	// Use a real client transport from mcp-go to avoid writing custom logic.
	targetClient := mcp.StreamableClientTransport{
		Endpoint:   serverURL,
		HTTPClient: httpClient,
		MaxRetries: -1,
	}
	// TODO(greedy52) this input context is used in conn.Close for sending
	// DELETE session request. Use TODO for now. It doesn't interfere with
	// Read/Write as they use the context passed to those calls.
	targetConn, err := targetClient.Connect(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	h := &HTTPReaderWriter{
		targetConn: targetConn,
		// Normally only one message at a time. Use a small buffer just in case.
		messagesToRead: make(chan jsonrpc.Message, 10),
	}

	go func() {
		for {
			msg, err := targetConn.Read(ctx)
			if err != nil {
				if !IsOKCloseError(err) {
					slog.WarnContext(ctx, "failed to read from target conn", "error", err)
				}
				break
			}
			h.messagesToRead <- msg
		}
	}()

	// TODO(greedy52) mcp.StreamableClientTransport does not expose the
	// "sessionUpdated" function to start listening stream. We need to manually
	// implement that before backporting sdk migration to a release.
	return h, nil
}

// WriteMessage sends out a HTTP request to target. WriteMessage implements
// MessageWriter.
func (h *HTTPReaderWriter) WriteMessage(ctx context.Context, msg jsonrpc.Message) error {
	return h.targetConn.Write(ctx, msg)
}

// Type implements TransportReader.
func (h *HTTPReaderWriter) Type() string {
	return types.MCPTransportHTTP
}

// ReadMessage returns responses and notifications received from the target.
// ReadMessage implements TransportReader.
func (h *HTTPReaderWriter) ReadMessage(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", io.EOF
	case msg := <-h.messagesToRead:
		data, err := jsonrpc.EncodeMessage(msg)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return string(data), nil
	}
}

// Close implements TransportReader.
func (h *HTTPReaderWriter) Close() error {
	return trace.Wrap(h.targetConn.Close())
}
