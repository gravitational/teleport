// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"

	"github.com/gravitational/teleport"
	authproto "github.com/gravitational/teleport/api/client/proto"
	tracessh "github.com/gravitational/teleport/api/observability/tracing/ssh"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

// WSConn is a gorilla/websocket minimal interface used by our web implementation.
// This interface exists to override the default websocket.Conn implementation,
// currently used by noopCloserWS to prevent WS being closed by wrapping stream.
type WSConn interface {
	Close() error

	LocalAddr() net.Addr
	RemoteAddr() net.Addr

	WriteControl(messageType int, data []byte, deadline time.Time) error
	WriteMessage(messageType int, data []byte) error
	ReadMessage() (messageType int, p []byte, err error)
	SetReadLimit(limit int64)
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error

	PongHandler() func(appData string) error
	SetPongHandler(h func(appData string) error)
	CloseHandler() func(code int, text string) error
	SetCloseHandler(h func(code int, text string) error)
}

// WSHandlerFunc specifies a handler that processes received a specific
// [Envelope] received via a web socket.
type WSHandlerFunc func(context.Context, Envelope)

// WSStream handles web socket communication with
// the frontend.
type WSStream struct {
	// encoder is used to encode UTF-8 strings.
	encoder *encoding.Encoder
	// decoder is used to decode UTF-8 strings.
	decoder *encoding.Decoder

	handlers map[string]WSHandlerFunc
	// once ensures that all channels are closed at most one time.
	once       sync.Once
	challengeC chan Envelope
	rawC       chan Envelope

	// buffer is a buffer used to store the remaining payload data if it did not
	// fit into the buffer provided by the callee to Read method
	buffer []byte

	// readMu protects reads to WSConn
	readMu sync.Mutex
	// writeMu protects writes to WSConn
	writeMu sync.Mutex
	// WSConn the connection to the UI
	WSConn

	// log holds the structured logger.
	log logrus.FieldLogger
}

// Replace \n with \r\n so the message is correctly aligned.
var replacer = strings.NewReplacer("\r\n", "\r\n", "\n", "\r\n")

func (t *WSStream) ReadMessage() (messageType int, p []byte, err error) {
	t.readMu.Lock()
	defer t.readMu.Unlock()
	return t.WSConn.ReadMessage()
}

func (t *WSStream) WriteMessage(messageType int, data []byte) error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	return t.WSConn.WriteMessage(messageType, data)
}

// WriteError displays an error in the terminal window.
func (t *WSStream) WriteError(msg string) {
	if _, writeErr := replacer.WriteString(t, msg); writeErr != nil {
		t.log.WithError(writeErr).Warnf("Unable to send error to terminal: %v", msg)
	}
}

func (t *WSStream) SetReadDeadline(deadline time.Time) error {
	return t.WSConn.SetReadDeadline(deadline)
}

func isOKWebsocketCloseError(err error) bool {
	return websocket.IsCloseError(err,
		websocket.CloseAbnormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseNormalClosure,
	)
}

func (t *WSStream) processMessages(ctx context.Context) {
	defer func() {
		t.close()
	}()
	t.WSConn.SetReadLimit(teleport.MaxHTTPRequestSize)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			ty, bytes, err := t.WSConn.ReadMessage()
			if err != nil {
				if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || isOKWebsocketCloseError(err) {
					return
				}

				msg := err.Error()
				if len(bytes) > 0 {
					msg = string(bytes)
				}
				select {
				case <-ctx.Done():
				default:
					t.WriteError(msg)
					return
				}
			}

			if ty != websocket.BinaryMessage {
				t.WriteError(fmt.Sprintf("Expected binary message, got %v", ty))
				return
			}

			var envelope Envelope
			if err := proto.Unmarshal(bytes, &envelope); err != nil {
				t.WriteError(fmt.Sprintf("Unable to parse message payload %v", err))
				return
			}

			switch envelope.Type {
			case defaults.WebsocketClose:
				return
			case defaults.WebsocketWebauthnChallenge:
				select {
				case <-ctx.Done():
					return
				case t.challengeC <- envelope:
				default:
				}
			case defaults.WebsocketRaw:
				select {
				case <-ctx.Done():
					return
				case t.rawC <- envelope:
				default:
				}
			default:
				if t.handlers == nil {
					continue
				}

				handler, ok := t.handlers[envelope.Type]
				if !ok {
					t.log.Warnf("Received web socket envelope with unknown type %v", envelope.Type)
					continue
				}

				go handler(ctx, envelope)
			}
		}
	}
}

// MFACodec converts MFA challenges/responses between their native types and a format
// suitable for being sent over a network connection.
type MFACodec interface {
	// Encode converts an MFA challenge to wire format
	Encode(chal *client.MFAAuthenticateChallenge, envelopeType string) ([]byte, error)

	// DecodeChallenge parses an MFA authentication challenge
	DecodeChallenge(bytes []byte, envelopeType string) (*authproto.MFAAuthenticateChallenge, error)

	// DecodeResponse parses an MFA authentication response
	DecodeResponse(bytes []byte, envelopeType string) (*authproto.MFAAuthenticateResponse, error)
}

// WriteChallenge encodes and writes the challenge to the
// websocket in the correct format.
func (t *WSStream) WriteChallenge(challenge *client.MFAAuthenticateChallenge, codec MFACodec) error {
	// Send the challenge over the socket.
	msg, err := codec.Encode(challenge, defaults.WebsocketWebauthnChallenge)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(t.WriteMessage(websocket.BinaryMessage, msg))
}

// ReadChallengeResponse reads and decodes the challenge response from the
// websocket in the correct format.
func (t *WSStream) ReadChallengeResponse(codec MFACodec) (*authproto.MFAAuthenticateResponse, error) {
	envelope, ok := <-t.challengeC
	if !ok {
		return nil, io.EOF
	}
	resp, err := codec.DecodeResponse([]byte(envelope.Payload), defaults.WebsocketWebauthnChallenge)
	return resp, trace.Wrap(err)
}

// ReadChallenge reads and decodes the challenge from the
// websocket in the correct format.
func (t *WSStream) ReadChallenge(codec MFACodec) (*authproto.MFAAuthenticateChallenge, error) {
	envelope, ok := <-t.challengeC
	if !ok {
		return nil, io.EOF
	}
	challenge, err := codec.DecodeChallenge([]byte(envelope.Payload), defaults.WebsocketWebauthnChallenge)
	return challenge, trace.Wrap(err)
}

// WriteAuditEvent encodes and writes the audit event to the
// websocket in the correct format.
func (t *WSStream) WriteAuditEvent(event []byte) error {
	// UTF-8 encode the error message and then wrap it in a raw envelope.
	encodedPayload, err := t.encoder.String(string(event))
	if err != nil {
		return trace.Wrap(err)
	}

	envelope := &Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketAudit,
		Payload: encodedPayload,
	}

	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		return trace.Wrap(err)
	}

	// Send bytes over the websocket to the web client.
	return trace.Wrap(t.WriteMessage(websocket.BinaryMessage, envelopeBytes))
}

// SSHSessionLatencyStats contain latency measurements for both
// legs of an ssh connection established via the Web UI.
type SSHSessionLatencyStats struct {
	// WebSocket measures the round trip time for a ping/pong via the websocket
	// established between the client and the Proxy.
	WebSocket int64 `json:"ws"`
	// SSH measures the round trip time for a keepalive@openssh.com request via the
	// connection established between the Proxy and the target host.
	SSH int64 `json:"ssh"`
}

// WriteLatency encodes and writes latency statistics.
func (t *WSStream) WriteLatency(latency SSHSessionLatencyStats) error {
	data, err := json.Marshal(latency)
	if err != nil {
		return trace.Wrap(err)
	}

	encodedPayload, err := t.encoder.String(string(data))
	if err != nil {
		return trace.Wrap(err)
	}

	envelope := &Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketLatency,
		Payload: encodedPayload,
	}

	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		return trace.Wrap(err)
	}

	// Send bytes over the websocket to the web client.
	return trace.Wrap(t.WriteMessage(websocket.BinaryMessage, envelopeBytes))
}

// Write wraps the data bytes in a raw envelope and sends.
func (t *WSStream) Write(data []byte) (n int, err error) {
	// UTF-8 encode data and wrap it in a raw envelope.
	encodedPayload, err := t.encoder.String(string(data))
	if err != nil {
		return 0, trace.Wrap(err)
	}
	envelope := &Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketRaw,
		Payload: encodedPayload,
	}
	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	// Send bytes over the websocket to the web client.
	err = t.WriteMessage(websocket.BinaryMessage, envelopeBytes)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return len(data), nil
}

// Read provides data received from [defaults.WebsocketRaw] envelopes. If
// the previous envelope was not consumed in the last read, any remaining data
// is returned prior to processing the next envelope.
func (t *WSStream) Read(out []byte) (int, error) {
	if len(t.buffer) > 0 {
		n := copy(out, t.buffer)
		if n == len(t.buffer) {
			t.buffer = []byte{}
		} else {
			t.buffer = t.buffer[n:]
		}
		return n, nil
	}

	envelope, ok := <-t.rawC
	if !ok {
		return 0, io.EOF
	}

	data, err := t.decoder.Bytes([]byte(envelope.Payload))
	if err != nil {
		return 0, trace.Wrap(err)
	}

	n := copy(out, data)
	// if the payload size is greater than [out], store the remaining
	// part in the buffer to be processed on the next Read call
	if len(data) > n {
		t.buffer = data[n:]
	}
	return n, nil
}

// sessionEndEvent is an event sent when a session ends.
type sessionEndEvent struct {
	// NodeID is the ID of the server where the session was created.
	NodeID string `json:"node_id"`
}

// SendCloseMessage sends a close message on the web socket.
func (t *WSStream) SendCloseMessage(id string) error {
	sessionMetadataPayload, err := json.Marshal(&sessionEndEvent{NodeID: id})
	if err != nil {
		return trace.Wrap(err)
	}

	envelope := &Envelope{
		Version: defaults.WebsocketVersion,
		Type:    defaults.WebsocketClose,
		Payload: string(sessionMetadataPayload),
	}
	envelopeBytes, err := proto.Marshal(envelope)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(t.WriteMessage(websocket.BinaryMessage, envelopeBytes))
}

func (t *WSStream) close() {
	t.once.Do(func() {
		close(t.rawC)
		close(t.challengeC)
	})
}

// Close sends a close message on the web socket and closes the web socket.
func (t *WSStream) Close() error {
	return trace.Wrap(t.WSConn.Close())
}

// Stream manages the [websocket.Conn] to the web UI
// for a terminal session.
type Stream struct {
	*WSStream

	// sshSession holds the "shell" SSH channel to the node.
	sshSession    *tracessh.Session
	sessionReadyC chan struct{}
}

// StreamConfig contains dependencies of a TerminalStream.
type StreamConfig struct {
	// The websocket to operate over. Required.
	WS WSConn
	// A logger to emit log messages. Optional.
	Logger logrus.FieldLogger
	// A custom set of handlers to process messages received
	// over the websocket. Optional.
	Handlers map[string]WSHandlerFunc
}

func NewWStream(ctx context.Context, ws WSConn, log logrus.FieldLogger, handlers map[string]WSHandlerFunc) *WSStream {
	w := &WSStream{
		log:        log,
		WSConn:     ws,
		encoder:    unicode.UTF8.NewEncoder(),
		decoder:    unicode.UTF8.NewDecoder(),
		rawC:       make(chan Envelope, 100),
		challengeC: make(chan Envelope, 1),
		handlers:   handlers,
	}

	go w.processMessages(ctx)

	return w
}

// NewStream creates a stream that manages reading and writing
// data over the provided [websocket.Conn]
func NewStream(ctx context.Context, cfg StreamConfig) *Stream {
	t := &Stream{
		sessionReadyC: make(chan struct{}),
	}

	if cfg.Handlers == nil {
		cfg.Handlers = map[string]WSHandlerFunc{}
	}

	if _, ok := cfg.Handlers[defaults.WebsocketResize]; !ok {
		cfg.Handlers[defaults.WebsocketResize] = t.handleWindowResize
	}

	if _, ok := cfg.Handlers[defaults.WebsocketFileTransferRequest]; !ok {
		cfg.Handlers[defaults.WebsocketFileTransferRequest] = t.handleFileTransferRequest
	}

	if _, ok := cfg.Handlers[defaults.WebsocketFileTransferDecision]; !ok {
		cfg.Handlers[defaults.WebsocketFileTransferDecision] = t.handleFileTransferDecision
	}

	if cfg.Logger == nil {
		cfg.Logger = utils.NewLogger()
	}

	t.WSStream = NewWStream(ctx, cfg.WS, cfg.Logger, cfg.Handlers)

	return t
}

// handleWindowResize receives window resize events and forwards
// them to the SSH session.
func (t *Stream) handleWindowResize(ctx context.Context, envelope Envelope) {
	select {
	case <-ctx.Done():
		return
	case <-t.sessionReadyC:
	}

	if t.sshSession == nil {
		return
	}

	var e map[string]interface{}
	err := json.Unmarshal([]byte(envelope.Payload), &e)
	if err != nil {
		t.log.Warnf("Failed to parse resize payload: %v", err)
		return
	}

	size, ok := e["size"].(string)
	if !ok {
		t.log.Errorf("expected size to be of type string, got type %T instead", size)
		return
	}

	params, err := session.UnmarshalTerminalParams(size)
	if err != nil {
		t.log.Warnf("Failed to retrieve terminal size: %v", err)
		return
	}

	// nil params indicates the channel was closed
	if params == nil {
		return
	}

	if err := t.sshSession.WindowChange(ctx, params.H, params.W); err != nil {
		t.log.Error(err)
	}
}

func (t *Stream) handleFileTransferDecision(ctx context.Context, envelope Envelope) {
	select {
	case <-ctx.Done():
		return
	case <-t.sessionReadyC:
	}

	if t.sshSession == nil {
		return
	}

	var e utils.Fields
	err := json.Unmarshal([]byte(envelope.Payload), &e)
	if err != nil {
		return
	}
	approved, ok := e["approved"].(bool)
	if !ok {
		t.log.Error("Unable to find approved status on response")
		return
	}

	if approved {
		err = t.sshSession.ApproveFileTransferRequest(ctx, e.GetString("requestId"))
	} else {
		err = t.sshSession.DenyFileTransferRequest(ctx, e.GetString("requestId"))
	}
	if err != nil {
		t.log.WithError(err).Error("Unable to respond to file transfer request")
	}
}

func (t *Stream) handleFileTransferRequest(ctx context.Context, envelope Envelope) {
	select {
	case <-ctx.Done():
		return
	case <-t.sessionReadyC:
	}

	if t.sshSession == nil {
		return
	}

	var e utils.Fields
	err := json.Unmarshal([]byte(envelope.Payload), &e)
	if err != nil {
		return
	}
	download, ok := e["download"].(bool)
	if !ok {
		t.log.Error("Unable to find download param in response")
		return
	}

	if err := t.sshSession.RequestFileTransfer(ctx, tracessh.FileTransferReq{
		Download: download,
		Location: e.GetString("location"),
		Filename: e.GetString("filename"),
	}); err != nil {
		t.log.WithError(err).Error("Unable to request file transfer")
	}
}

func (t *Stream) SessionCreated(s *tracessh.Session) {
	t.sshSession = s
	close(t.sessionReadyC)
}

func (t *Stream) Close() error {
	if t.sshSession != nil {
		return trace.NewAggregate(t.sshSession.Close(), t.WSStream.Close())
	} else {
		return trace.Wrap(t.WSStream.Close())
	}
}
