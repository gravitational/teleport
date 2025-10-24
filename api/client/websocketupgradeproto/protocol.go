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
	"os"
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
		// Read only the frame header first - never read the full body into memory
		header, err := ws.ReadHeader(c.underlyingConn)
		if err != nil {
			c.mu.Lock()
			if !c.localClosed {
				c.remoteClosed = true
			}
			c.cond.Broadcast()
			c.mu.Unlock()
			return
		}

		switch header.OpCode {
		case ws.OpClose:
			// Read and discard the close frame payload (if any)
			if header.Length > 0 {
				_, err := io.CopyN(io.Discard, c.underlyingConn, header.Length)
				if err != nil {
					c.mu.Lock()
					if !c.localClosed {
						c.remoteClosed = true
					}
					c.cond.Broadcast()
					c.mu.Unlock()
					return
				}
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
		case ws.OpBinary:
			// Read the binary payload in chunks, copying directly to receive buffer
			// Never read the full body into memory
			remaining := header.Length
			for remaining > 0 {
				c.mu.Lock()
				// Wait for a reader to be ready
				for !c.receiveBuffer.operate && !c.localClosed && !c.remoteClosed {
					c.cond.Wait()
				}

				if c.localClosed || c.remoteClosed {
					c.mu.Unlock()
					// Discard remaining bytes to keep protocol in sync
					if remaining > 0 {
						io.CopyN(io.Discard, c.underlyingConn, remaining)
					}
					return
				}

				// Read directly into the reader's buffer - no intermediate allocation
				toRead := remaining
				if toRead > int64(len(c.receiveBuffer.data)) {
					toRead = int64(len(c.receiveBuffer.data))
				}

				n, err := io.ReadFull(c.underlyingConn, c.receiveBuffer.data[:toRead])
				if err != nil {
					if !c.localClosed {
						c.remoteClosed = true
					}
					c.cond.Broadcast()
					c.mu.Unlock()
					return
				}

				// Unmask the data if needed
				if header.Masked {
					ws.Cipher(c.receiveBuffer.data[:n], header.Mask, int(header.Length-remaining))
				}

				remaining -= int64(n)
				c.receiveBuffer.length = n
				c.receiveBuffer.operate = false
				c.cond.Broadcast()
				c.mu.Unlock()
			}
		case ws.OpPong:
			// Read and discard the pong payload
			//  no action needed beyond consuming the frame to keep the protocol in sync.
			if header.Length > 0 {
				_, err := io.CopyN(io.Discard, c.underlyingConn, header.Length)
				if err != nil {
					c.mu.Lock()
					if !c.localClosed {
						c.remoteClosed = true
					}
					c.cond.Broadcast()
					c.mu.Unlock()
					return
				}
			}
		case ws.OpPing:
			// Read the ping payload to respond with a pong
			var payload []byte
			// Limit the maximum size of the ping payload to avoid excessive memory allocation
			// Teleport only uses small ping payloads anyway.
			const maxPingPayloadSize = 250
			if header.Length > 0 {
				payload = make([]byte, min(header.Length, maxPingPayloadSize))
				_, err := io.ReadFull(c.underlyingConn, payload)
				if err != nil {
					c.mu.Lock()
					if !c.localClosed {
						c.remoteClosed = true
					}
					c.cond.Broadcast()
					c.mu.Unlock()
					return
				}
				// Unmask the payload if needed
				if header.Masked {
					ws.Cipher(payload, header.Mask, 0)
				}
				if header.Length > int64(maxPingPayloadSize) {
					// Discard remaining bytes to keep protocol in sync
					_, err := io.CopyN(io.Discard, c.underlyingConn, header.Length-int64(maxPingPayloadSize))
					if err != nil {
						c.mu.Lock()
						if !c.localClosed {
							c.remoteClosed = true
						}
						c.cond.Broadcast()
						c.mu.Unlock()
						return
					}
				}
			}

			c.mu.Lock()
			// Respond to Ping frames with a Pong frame containing the same payload.
			// Pong frames are queued to be sent after waking up the write loop.
			pongFrame := ws.NewPongFrame(payload)
			c.pingPongReplies = append(c.pingPongReplies, pongFrame)
			c.cond.Broadcast()
			c.mu.Unlock()
		}
	}
}

// writeLoop continuously sends WebSocket frames from the send buffer to the
// underlying connection. It manages transmission of binary data, ping/pong
// responses, and close frames as required. The loop ends when the connection
// is closed locally or remotely, ensuring a close frame is sent if applicable.
// It also releases the read loop by setting a read deadline when needed.
func (c *Conn) writeLoop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// unblockReaderOnErr is a helper function that sets a past read deadline
	// on the underlying connection to unblock the read loop in case of errors
	// during writing. It also broadcasts the condition variable to wake up
	// any waiting goroutines.
	unblockReaderOnErr := func() {
		if c.localClosed {
			c.remoteClosed = true
		}
		pastTime := time.Now().Add(-1 * time.Second)
		c.underlyingConn.SetReadDeadline(pastTime)
		c.cond.Broadcast()
	}

	for {
		// Check if there's data waiting to be sent
		if c.sendBuffer.operate {

			bytesToWrite := len(c.sendBuffer.data)
			// Clear operate flag so Write() knows we've taken ownership
			// But don't set completed yet - that happens after the actual write
			c.sendBuffer.operate = false
			c.cond.Broadcast()

			err := c.writeFrame(ws.NewBinaryFrame(c.sendBuffer.data))

			// Now mark as completed and set the length
			c.sendBuffer.length = bytesToWrite
			c.sendBuffer.completed = true
			if errors.Is(err, os.ErrDeadlineExceeded) && c.localClosed {
			} else if err != nil {
				unblockReaderOnErr()
				return
			} else {

				c.cond.Broadcast()
				continue
			}
		}

		// Handle ping/pong replies
		if len(c.pingPongReplies) > 0 {
			c.underlyingConn.SetWriteDeadline(time.Time{})
			frames := c.pingPongReplies
			c.pingPongReplies = nil

			for _, fr := range frames {
				if err := c.writeFrame(fr); err != nil {
					unblockReaderOnErr()
					return
				}
			}
			c.cond.Broadcast()
			// After sending ping/pong replies, continue to check for more data to send.
			// This is required because in writeFrame we unlock the mutex, so there might be new data
			// to send.
			continue
		}

		if c.localClosed || c.remoteClosed {
			c.underlyingConn.SetWriteDeadline(time.Now().Add(time.Second))
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
			if err := c.writeFrame(closeFrame); err != nil {
				unblockReaderOnErr()
				return
			}

			// If the connection doesn't support the close process, we can return
			// immediately after sending the close frame as the server will never
			// respond with a close frame and we can unblock the read loop.
			// This is to avoid holding the connection open waiting for a close frame
			// that we know will never come.
			if !c.supportsCloseProccess {
				unblockReaderOnErr()
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

// writeFrame writes a WebSocket frame to the underlying connection.
// The mutex must be held when calling this function. It temporarily unlocks
// the mutex during the write operation to avoid blocking other goroutines,
// then re-locks it before returning.
func (c *Conn) writeFrame(frame ws.Frame) error {
	// If the connection is a client connection, we must mask the frame
	// as per the WebSocket protocol. We use an empty mask for simplicity
	// so messages are not actually masked, but the masked bit is set.
	frame.Header.Masked = c.connType == clientConnection

	// Unlock the mutex while writing to avoid blocking other operations.
	// The mutex is re-locked after the write is complete.
	c.mu.Unlock()
	defer c.mu.Lock()
	return ws.WriteFrame(c.underlyingConn, frame)
}

// WritePing queues a Ping frame to be sent to the remote peer.
// Note: Client connections never send pings, only servers do.
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
	c.mu.Lock()
	c.closeFrame = &frame
	c.underlyingConn.SetWriteDeadline(time.Now().Add(-1 * time.Second))

	err := c.managedConn.closeLocked()

	c.mu.Unlock()

	// Signal the write loop to send the close frame.

	// Wait for the read and write loops to finish.
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
