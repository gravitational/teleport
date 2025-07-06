/*
Copyright 2025 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package websocketupgradeproto

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/gobwas/ws"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
)

const (
	ComponentTeleport = "teleport"
	ComponentClient   = "client"
)

// connectionType represents the side of the connection.
// It can be either a server or a client connection.
// This is used to determine how to handle WebSocket frames, especially
// for the ping/pong frames.
type connectionType int

const (
	// serverConnection indicates that this is a server-side connection.
	serverConnection connectionType = iota + 1
	// clientConnection indicates that this is a client-side connection.
	clientConnection
)

var _ net.Conn = (*WebsocketUpgradeConn)(nil)

// WebsocketUpgradeConn is a WebSocket connection that supports the WebSocket
// upgrade protocol. It implements the net.Conn interface and provides methods
// to read and write WebSocket frames. It also supports the WebSocket close
// protocol, allowing it to gracefully close the connection with a close frame.
type WebsocketUpgradeConn struct {
	conn       net.Conn
	readBuffer []byte
	readError  error
	readMutex  sync.Mutex
	writeMutex sync.Mutex

	logContext            context.Context
	logger                *slog.Logger
	once                  sync.Once
	supportsCloseProtocol bool
	connType              connectionType
	protocol              string
}

// newWebsocketUpgradeConnConfig holds the configuration for creating a new
// WebsocketUpgradeConn. It includes the context, connection, logger, handshake
// information, and the type of connection (server or client).
type newWebsocketUpgradeConnConfig struct {
	ctx      context.Context
	conn     net.Conn
	logger   *slog.Logger
	hs       ws.Handshake
	connType connectionType
}

// newWebsocketUpgradeConn creates a new WebsocketUpgradeConn instance.
// It initializes the connection with the provided configuration, including
// the context, connection, logger, and handshake information.
func newWebsocketUpgradeConn(cfg newWebsocketUpgradeConnConfig) *WebsocketUpgradeConn {
	return &WebsocketUpgradeConn{
		logContext:            cfg.ctx,
		logger:                cfg.logger,
		conn:                  cfg.conn,
		supportsCloseProtocol: cfg.hs.Protocol == constants.WebAPIConnUpgradeProtocolWebSocketClose,
		connType:              cfg.connType,
		protocol:              cfg.hs.Protocol,
	}
}

func (c *WebsocketUpgradeConn) NetConn() net.Conn {
	return c.conn
}

func (c *WebsocketUpgradeConn) Read(b []byte) (int, error) {
	c.readMutex.Lock()
	defer c.readMutex.Unlock()

	n, err := c.readLocked(b)

	// Timeout errors can be temporary. For example, when this connection is
	// passed to the kube TLS server, it may get "hijacked" again. During the
	// hijack, the SetReadDeadline is called with a past timepoint to fail this
	// Read so that the HTTP server's background read can be stopped. In such
	// cases, return the original net.Error and clear the cached read error.
	var netError net.Error
	if errors.As(err, &netError) && netError.Timeout() {
		c.readError = nil
		return n, netError
	}
	return n, err
}

func (c *WebsocketUpgradeConn) readLocked(b []byte) (int, error) {
	if len(c.readBuffer) > 0 {
		n := copy(b, c.readBuffer)
		if n < len(c.readBuffer) {
			c.readBuffer = c.readBuffer[n:]
		} else {
			c.readBuffer = nil
		}
		return n, nil
	}

	// Stop reading if any previous read err.
	if c.readError != nil {
		return 0, c.readError
	}

	for {
		frame, err := ws.ReadFrame(c.conn)
		if err != nil {
			c.readError = err
			return 0, err
		}

		// All client frames should be masked.
		if frame.Header.Masked {
			frame = ws.UnmaskFrame(frame)
		}

		switch frame.Header.OpCode {
		case ws.OpClose:
			// If we receive a close frame, we should respond with a close frame.
			if c.supportsCloseProtocol {
				// Run the close protocol only once to avoid sending frames again
				// when closing the connection.
				c.once.Do(func() {
					if err := c.writeFrame(
						ws.NewCloseFrame(
							ws.NewCloseFrameBody(ws.StatusNormalClosure, ""),
						),
					); err != nil {
						if !isOkNetworkErrOrTimeout(err) {
							c.logger.DebugContext(c.logContext, "error writing close frame", "error", err)
						}
					}
				})
			}
			c.readError = io.EOF
			return 0, io.EOF
		case ws.OpBinary:
			c.readBuffer = frame.Payload
			return c.readLocked(b)
		case ws.OpPong:
			// Receives Pong as response to Ping. Nothing to do.
		case ws.OpPing:
			if c.connType == serverConnection {
				continue
			}
			// If this is a client connection, we should respond with a Pong frame.
			pongFrame := ws.NewPongFrame(frame.Payload)
			if err := c.writeFrame(pongFrame); err != nil {
				c.logger.DebugContext(c.logContext, "error writing Pong frame", "error", err)
				return 0, err
			}
		}
	}
}

// websocketCloseProtocol implements the WebSocket close protocol.
// It sends a close frame to the client and waits for a close frame response.
// This is important to ensure that the client receives the close frame and
// can gracefully close the connection.
// If the client does not respond, the connection will be closed after a deadline expires.
// The acquireReadLock parameter indicates whether to acquire the read lock
// before waiting for the close frame response. This is useful to prevent
// concurrent reads while waiting for the close frame.
func (c *WebsocketUpgradeConn) websocketCloseProtocol(closeCode ws.StatusCode, closeText string) {
	if !c.supportsCloseProtocol {
		return
	}
	c.once.Do(func() {
		// Set a deadline to reset any previously set deadline.
		// Also, this is important to ensure that the connection
		// won't hang indefinitely if the client does not respond to the close frame.
		const deadline = 3 * time.Second
		if err := c.SetDeadline(time.Now().Add(deadline)); err != nil {
			if !isOkNetworkErrOrTimeout(err) {
				c.logger.DebugContext(c.logContext, "error setting read deadline", "error", err)
			}
			return
		}
		// Per RFC 6455, the side initiating the close should send a close frame
		// and then wait for the close frame from the other side.
		// If the other side does not respond, the connection will be closed after
		// the deadline expires.
		closeFrame := ws.NewCloseFrame(
			ws.NewCloseFrameBody(closeCode, closeText),
		)

		if err := c.writeFrame(closeFrame); err != nil {
			if !isOkNetworkErrOrTimeout(err) {
				c.logger.DebugContext(c.logContext, "error writing close frame", "error", err)
			}
			return
		}
	})

	c.readMutex.Lock()
	defer c.readMutex.Unlock()
	var (
		tmpData  [50]byte
		fullData []byte
	)
	// Wait for the close frame from the other side.
	// This will block until the close frame is received or the deadline expires.
	for {
		n, err := c.readLocked(tmpData[:])
		if err != nil {
			break
		}
		fullData = append(fullData, tmpData[:n]...)
	}

	c.readBuffer = fullData
}

// writeFrame writes a WebSocket frame to the connection.
// It locks the write mutex to ensure that only one goroutine can write to the
// connection at a time. This is important to prevent
// concurrent writes between the ping goroutine and the main connection
// handling goroutine.
func (c *WebsocketUpgradeConn) writeFrame(frame ws.Frame) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()

	// If the connection is a client connection, we should mask the frame.
	frame.Header.Masked = c.connType == clientConnection

	// There is no need to mask from server to client.
	return ws.WriteFrame(c.conn, frame)
}

// Write writes a binary frame to the connection.
func (c *WebsocketUpgradeConn) Write(b []byte) (n int, err error) {
	binaryFrame := ws.NewBinaryFrame(b)
	if err := c.writeFrame(binaryFrame); err != nil {
		return 0, err
	}
	return len(b), nil
}

// WritePing sends a Ping frame to the client.
func (c *WebsocketUpgradeConn) WritePing() error {
	if c.connType == clientConnection {
		return nil
	}

	pingFrame := ws.NewPingFrame([]byte(ComponentTeleport))
	return trace.Wrap(c.writeFrame(pingFrame))
}

// Close closes the connection gracefully with a normal closure status.
// This method is used to close the connection without any specific status code
// or message, indicating that the connection is being closed normally.
func (c *WebsocketUpgradeConn) Close() error {
	return c.CloseWithStatus(ws.StatusNormalClosure, "")
}

// CloseWithStatus closes the connection with a specific status code and message.
// This method is used to gracefully close the connection, ensuring that the
// client receives a close frame if it supports the WebSocket close protocol.
func (c *WebsocketUpgradeConn) CloseWithStatus(status ws.StatusCode, message string) error {
	// If the client supports the close protocol, we should send a close frame
	// before closing the connection. This is important to ensure that the client
	// receives the close frame and can gracefully close the connection.
	c.websocketCloseProtocol(status, message)

	// Close the underlying connection. This will also cancel the ping goroutine
	// if it is running.
	if err := c.conn.Close(); err != nil && !isOkNetworkErrOrTimeout(err) {
		c.logger.DebugContext(c.logContext, "error closing connection", "error", err)
		return err
	}
	return nil
}

// SetDeadline sets the deadline for the connection.
func (c *WebsocketUpgradeConn) SetDeadline(t time.Time) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	return c.conn.SetDeadline(t)
}

// SetWriteDeadline sets the write deadline for the connection.
func (c *WebsocketUpgradeConn) SetWriteDeadline(t time.Time) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	return c.conn.SetWriteDeadline(t)
}

// SetReadDeadline sets the read deadline for the connection.
func (c *WebsocketUpgradeConn) SetReadDeadline(t time.Time) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	return c.conn.SetReadDeadline(t)
}

// LocalAddr returns the local address of the connection.
func (c *WebsocketUpgradeConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr returns the remote address of the connection.
func (c *WebsocketUpgradeConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// Protocol returns the WebSocket subprotocol negotiated during the handshake.
func (c *WebsocketUpgradeConn) Protocol() string {
	return c.protocol
}
