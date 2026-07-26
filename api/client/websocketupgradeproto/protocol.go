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
	"io"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/gobwas/ws"

	"github.com/gravitational/teleport/api/constants"
)

const (
	// ComponentTeleport is the name of the Teleport server component.
	ComponentTeleport = "teleport"
	// ComponentClient is the name of the Teleport client component.
	ComponentClient = "client"
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

const (
	// maxDeadlineOnClose is the time to wait for the close handshake to complete
	// before forcefully closing the underlying connection.
	maxDeadlineOnClose = 3 * time.Second
)

var _ interface {
	net.Conn
	NetConn() net.Conn
} = (*Conn)(nil)

// Conn represents a WebSocket connection that implements the net.Conn interface
// backed by an underlying WebSocket connection to bypass proxies and load balancers
// that terminate TLS connections between the client and server.
// It provides methods for reading and writing WebSocket frames, managing control
// frames (Ping, Pong, and Close), and handling connection state.
//
// The Conn type ensures compliance with the WebSocket close handshake, allowing
// for graceful termination of the connection while maintaining protocol integrity
// and proper resource cleanup.
type Conn struct {
	managedConn

	// underlyingConn is the underlying network after the WebSocket upgrade.
	underlyingConn net.Conn
	// logContext is the context used for logging.
	logContext context.Context
	// logger is the logger used for logging.
	logger *slog.Logger
	// connType indicates whether this is a server or client connection.
	// This affects how ping/pong frames are handled as clients never send pings
	// and servers never respond to pongs.
	connType connectionType
	// protocol is the negotiated WebSocket sub-protocol for this connection.
	protocol string
	// supportsCloseProccess indicates whether this connection supports
	// the WebSocket close process.
	supportsCloseProccess bool

	// pingPongReplies holds the ping/pong frames to be sent in the next write loop iteration.
	pingPongReplies []ws.Frame
	// closeFrame is the close frame to send when closing the connection.
	// If nil and the connection should be closed, a normal closure frame will be sent.
	closeFrame *ws.Frame

	// wg is used to wait for the read and write loops to finish during Close().
	wg sync.WaitGroup
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
func newWebsocketUpgradeConn(cfg newWebsocketUpgradeConnConfig) *Conn {
	conn := &Conn{
		managedConn: managedConn{
			localAddr:  cfg.conn.LocalAddr(),
			remoteAddr: cfg.conn.RemoteAddr(),
		},
		logContext:            cfg.ctx,
		logger:                cfg.logger,
		underlyingConn:        cfg.conn,
		connType:              cfg.connType,
		protocol:              cfg.hs.Protocol,
		supportsCloseProccess: cfg.hs.Protocol == constants.WebAPIConnUpgradeProtocolWebSocketClose,
	}
	conn.cond.L = &conn.mu

	// TODO(tigrato): transform this into wg.Go once we upgrade to Go 1.25.
	conn.wg.Add(2)
	go func() {
		defer conn.wg.Done()
		conn.writeLoop()
	}()

	go func() {
		defer conn.wg.Done()
		conn.readLoop()
	}()

	return conn
}

func (c *Conn) NetConn() net.Conn {
	return c.underlyingConn
}

// readLoop continuously reads WebSocket frames from the underlying connection
// and processes them based on their OpCode. It handles the following frame types:
//   - Binary: stored in the receive buffer and signals waiting readers.
//   - Ping/Pong: responds to Ping frames, ignores Pong frames as they are replies.
//   - Close: triggers the WebSocket close handshake if not already initiated locally.
//
// The loop terminates when an error occurs or when the connection is closed
// gracefully by either the local or remote endpoint.
func (c *Conn) readLoop() {
	defer func() {
		c.underlyingConn.Close()
		c.cond.Broadcast()
	}()
	for {
		header, err := ws.ReadHeader(c.underlyingConn)
		if err != nil {
			c.handleReadError()
			return
		}

		switch header.OpCode {
		case ws.OpClose:
			if err := c.drainFramePayload(header); err != nil {
				c.handleReadError()
				return
			}

			c.mu.Lock()
			// If we already closed the connection locally, this message is
			// the acknowledgment of our close frame, so we can just return.
			// returning will close the underlying connection and end the read loop.
			if c.localClosed {
				c.mu.Unlock()
				return
			}

			// If we receive a close frame from the remote side,
			// we need to respond with a close frame and close the connection.
			// We set remoteClosed to true to indicate that the remote side
			// has started the close process and we broadcast the cond to
			// wake up the write loop to send the close frame.
			c.remoteClosed = true
			c.cond.Broadcast()
			c.mu.Unlock()
		case ws.OpBinary, ws.OpContinuation:
			if err := c.readBinaryPayloadToBuffer(header); err != nil {
				c.handleReadError()
				return
			}
		case ws.OpPong:
			// Pong frames are replies to Ping frames sent by us and should be ignored.
			// Just read and discard the Pong frame payload.
			if err := c.drainFramePayload(header); err != nil {
				c.handleReadError()
				return
			}
		case ws.OpPing:
			// Read the Ping frame payload. Ping frames are control messages
			// and their payloads are limited to 125 bytes.
			// We need to read the payload to respond with a Pong frame because
			// the Pong frame should contain the same payload as the Ping frame.
			payload, err := c.readControlPayload(header)
			if err != nil {
				c.handleReadError()
				return
			}

			c.mu.Lock()
			// Respond to Ping frames with a Pong frame containing the same payload.
			// Pong frames are queued to be sent after waking up the write loop.
			pongFrame := ws.NewPongFrame(payload)
			c.pingPongReplies = append(c.pingPongReplies, pongFrame)
			c.cond.Broadcast()
			c.mu.Unlock()
		default:
			// For other frame types, just drain the payload as we don't process them.
			if err := c.drainFramePayload(header); err != nil {
				c.handleReadError()
				return
			}
		}
	}
}

// handleReadError handles errors that occur during reading from the WebSocket connection.
// It sets the remoteClosed flag if the connection is not already closed locally
// and broadcasts the condition variable to wake up any waiting goroutines.
func (c *Conn) handleReadError() {
	c.mu.Lock()
	if !c.localClosed {
		c.remoteClosed = true
	}
	c.cond.Broadcast()
	c.mu.Unlock()
}

// readBinaryPayloadToBuffer reads the binary payload of a WebSocket data frame
// (or continuation frame) directly into the connection’s internal receive buffer.
//
// It ensures that the entire payload is read before returning and never produces
// partial reads. If any error occurs during reading (such as an I/O or closure error),
// the function aborts and returns that error.
//
// This function also handles masked frames, applying the WebSocket masking key
// during reading as specified in RFC 6455 directly into the receive buffer.
// Webscoket masks do not change the length of the payload, so no adjustments are needed.
//
// It ensures that the receive buffer [c.receiveBuffer] never exceeds its configured
// capacity [receiveBufferSize]. If the buffer is full, it waits until space becomes
// available before reading more data.
//
// Behavior Summary:
//   - Fully reads the frame payload or returns an error.
//   - Waits for buffer space when the receive buffer is full.
//   - Handles masked frames correctly according to WebSocket protocol.
//   - Respects connection closure state (`c.localClosed`).
//   - Signals other goroutines when new data becomes available.
func (c *Conn) readBinaryPayloadToBuffer(header ws.Header) error {
	remaining := header.Length
	if remaining == 0 {
		return nil
	}

	offset := 0
	c.mu.Lock()
	for remaining > 0 {
		// receiveBuffer is full, wait for space to become available.
		// Next time .Read is called, it will read from the buffer
		// and free up space and signal this condition variable.
		for c.receiveBuffer.len() >= receiveBufferSize {
			if c.localClosed {
				c.mu.Unlock()
				return net.ErrClosed
			}
			c.cond.Wait()
		}

		if c.localClosed {
			c.mu.Unlock()
			return net.ErrClosed
		}

		space := min(
			receiveBufferSize-int(c.receiveBuffer.len()),
			int(remaining),
		)

		if space == 0 {
			continue
		}

		// we call reserve to ensure there is enough space in the buffer
		// but space is never more than receiveBufferSize - len(receiveBuffer)
		// so this will never allocate more than that.
		c.receiveBuffer.reserve(uint64(space))

		// Get the free slices from the receive buffer.
		f1, f2 := c.receiveBuffer.free()

		// Read into the first slice.
		// First slice is guaranteed to have some space.
		chunk := min(len(f1), space)
		buf := f1[:chunk]
		if err := c.readFramePartialPayloadLocked(buf); err != nil {
			c.mu.Unlock()
			return err
		}
		if header.Masked && chunk > 0 {
			ws.Cipher(buf, header.Mask, offset)
		}
		offset += chunk
		c.receiveBuffer.grow(uint64(chunk))

		remaining -= int64(chunk)

		if len(f2) > 0 {
			chunk := min(len(f2), space-chunk)
			buf := f2[:chunk]
			if err := c.readFramePartialPayloadLocked(buf); err != nil {
				c.mu.Unlock()
				return err
			}
			if header.Masked && chunk > 0 {
				ws.Cipher(buf, header.Mask, offset)
			}
			offset += chunk
			c.receiveBuffer.grow(uint64(chunk))
			remaining -= int64(chunk)
		}

		c.cond.Broadcast()

	}
	c.mu.Unlock()

	return nil
}

// readFramePartialPayloadLocked reads a portion of the frame payload of len(buf) into the provided buffer.
// It assumes that the connection mutex is already locked and unlocks it during the read
// to avoid blocking other write operations. After reading, it re-locks the mutex.
// It's the caller's responsibility to ensure that the buffer size does not exceed
// the remaining payload length.
// It's safe to unlock the mutex here because the readLoop is the only
// goroutine that reads from the underlying connection and advances the buffer's .end field.
// The conn.Read call only updates the .start field, which means it can only
// increase the available space in the buffer, never reduce it—so there’s no risk of race conditions.
func (c *Conn) readFramePartialPayloadLocked(buf []byte) error {
	// Unlock the mutex while reading to avoid blocking other operations.
	c.mu.Unlock()
	defer c.mu.Lock()
	_, err := io.ReadFull(c.underlyingConn, buf)
	return err
}

// readControlPayload reads the payload of a control frame (Ping, Pong, Close).
// It ensures that the payload length does not exceed the maximum allowed size
// for control frames (125 bytes) and returns an error if it does.
// The function reads the entire payload into a byte slice and returns it.
func (c *Conn) readControlPayload(header ws.Header) ([]byte, error) {
	if header.Length < 0 || header.Length > ws.MaxControlFramePayloadSize {
		return nil, ws.ErrProtocolControlPayloadOverflow
	}

	payload := make([]byte, header.Length)
	n, err := io.ReadFull(c.underlyingConn, payload)
	return payload[:n], err
}

// drainFramePayload discards the payload of a WebSocket frame by reading
// and ignoring it. This is used for frames where the payload is not needed,
// such as control frames that are not processed further.
func (c *Conn) drainFramePayload(header ws.Header) error {
	// Discard the frame payload by reading it without processing.
	_, err := io.CopyN(io.Discard, c.underlyingConn, header.Length)
	return err
}

// writeLoop continuously sends WebSocket frames from the send buffer to the
// underlying connection. It manages transmission of binary data, ping/pong
// responses, and close frames as required. The loop ends when the connection
// is closed locally or remotely, ensuring a close frame is sent if applicable.
// It also releases the read loop by setting a read deadline when needed.
func (c *Conn) writeLoop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// unblockReaderOnErrLocked is a helper function that sets a past read deadline
	// on the underlying connection to unblock the read loop in case of errors
	// during writing. It also broadcasts the condition variable to wake up
	// any waiting goroutines.
	unblockReaderOnErrLocked := func() {
		if c.localClosed {
			c.remoteClosed = true
		}
		pastTime := time.Now().Add(-1 * time.Second)
		c.underlyingConn.SetReadDeadline(pastTime)
		c.cond.Broadcast()
	}

	// maxFrameSize is the maximum amount of data that can be transmitted at once;
	// picked for sanity's sake, and to allow acks to be sent relatively frequently.
	dataBuffer := make([]byte, bufferSize)
	for {
		n := c.sendBuffer.read(dataBuffer)
		if n > 0 {
			err := c.writeFrameUnlocking(
				ws.NewBinaryFrame(dataBuffer[:n]),
			)
			if err != nil {
				unblockReaderOnErrLocked()
				return
			}
			c.cond.Broadcast()
			continue
		}

		// Handle ping/pong replies
		if len(c.pingPongReplies) > 0 {
			frame := c.pingPongReplies[0]
			// Remove the frame from the slice
			c.pingPongReplies = c.pingPongReplies[1:]

			if err := c.writeFrameUnlocking(frame); err != nil {
				unblockReaderOnErrLocked()
				return
			}

			c.cond.Broadcast()
			// After sending ping/pong replies, continue to check for more data to send.
			// This is required because in writeFrame we unlock the mutex, so there might be new data
			// to send.
			continue
		}

		if c.localClosed || c.remoteClosed {
			var closeFrame ws.Frame
			if c.closeFrame != nil {
				closeFrame = *c.closeFrame
			} else {
				closeFrame = ws.NewCloseFrame(
					ws.NewCloseFrameBody(ws.StatusNormalClosure, ""),
				)
			}

			// always try to send a close frame when closing the connection
			// even if the connection does not support the close process.
			if err := c.writeFrameUnlocking(closeFrame); err != nil {
				unblockReaderOnErrLocked()
				return
			}

			// If the connection doesn't support the close process, we can return
			// immediately after sending the close frame as the server will never
			// respond with a close frame and we can unblock the read loop.
			// This is to avoid holding the connection open waiting for a close frame
			// that we know will never come.
			if !c.supportsCloseProccess {
				unblockReaderOnErrLocked()
				return
			}

			// Set a read deadline to avoid blocking forever waiting for
			// the close frame from the remote side that may never come.
			const deadline = 3 * time.Second
			c.underlyingConn.SetReadDeadline(time.Now().Add(deadline))

			// Once we write the close frame, we can return from this loop.
			// We will never need to write anything else into the connection,
			// so we can just wait for the read loop to read the close frame
			// from the remote side and close the connection.
			return
		}

		c.cond.Wait()
	}
}

// writeFrameLocked writes a WebSocket frame to the connection without acquiring
// the write mutex. This is used when we already hold the write mutex.
func (c *Conn) writeFrameUnlocking(frame ws.Frame) error {
	// If the connection is a client connection, we should mask the frame
	// as per the WebSocket protocol. In this case, we use a empty mask
	// for simplicity so messages are not actually masked, but the masked bit is set.
	frame.Header.Masked = c.connType == clientConnection

	// Unlock the mutex while writing to avoid blocking other operations.
	// The mutex is re-locked after the write is complete.
	c.mu.Unlock()
	defer c.mu.Lock()
	// There is no need to mask from server to client.
	return ws.WriteFrame(c.underlyingConn, frame)
}

// WritePing sends a Ping frame to the client.
func (c *Conn) WritePing() error {
	// Clients never send pings.
	if c.connType == clientConnection {
		return nil
	}

	// Create a Ping frame with the Teleport component as payload.
	pingFrame := ws.NewPingFrame([]byte(ComponentTeleport))

	c.mu.Lock()
	defer c.mu.Unlock()
	// Queue the ping frame to be sent in the next write loop iteration
	// and signal the write loop.
	c.pingPongReplies = append(c.pingPongReplies, pingFrame)
	c.cond.Broadcast()
	return nil
}

// Protocol returns the negotiated WebSocket sub-protocol for this connection.
func (c *Conn) Protocol() string {
	return c.protocol
}

// Close closes the WebSocket connection gracefully by sending a close frame
// to the remote side if supported.
func (c *Conn) Close() error {
	closeFrame := ws.NewCloseFrame(
		ws.NewCloseFrameBody(ws.StatusNormalClosure, ""),
	)
	return c.closeWithErrFrame(closeFrame)
}

func (c *Conn) closeWithErrFrame(frame ws.Frame) error {
	// This afterFunc sets a deadline to unblock the read and write loops in case
	// the remote side never acknowledges previously sent frames and the
	// write loop gets stuck trying to send any frame.
	// If the remote side does not respond, the deadline will trigger and
	// the read and write loops will exit. In such cases, we don't properly
	// complete the close handshake, but connection wasn't healthy anyway.
	stopTimer := time.AfterFunc(
		maxDeadlineOnClose,
		func() {
			_ = c.underlyingConn.Close()
		},
	)
	// This defer will ensure that the deadline is cleared when the function returns.
	defer stopTimer.Stop()

	c.mu.Lock()
	c.closeFrame = &frame
	// Signal the write loop to send the close frame.
	err := c.managedConn.closeLocked()
	c.mu.Unlock()

	// Wait for the read and write loops to finish.
	// This will wait at most until the deadline [maxDeadlineOnClose] set above triggers.
	c.wg.Wait()

	return err
}

// CloseWithStatus closes the WebSocket connection with the given status code and message.
func (c *Conn) CloseWithStatus(code ws.StatusCode, message string) error {
	closeFrame := ws.NewCloseFrame(
		ws.NewCloseFrameBody(code, message),
	)
	return c.closeWithErrFrame(closeFrame)
}
