/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package web

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/gobwas/ws"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils/pingconn"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// selectConnectionUpgrade selects the requested upgrade type and returns the
// corresponding handler.
func (h *Handler) selectConnectionUpgrade(r *http.Request) (string, ConnectionHandler, error) {
	upgrades := append(
		r.Header.Values(constants.WebAPIConnUpgradeTeleportHeader),
		r.Header.Values(constants.WebAPIConnUpgradeHeader)...,
	)

	// Prefer WebSocket when multiple types are provided.
	switch {
	case slices.Contains(upgrades, constants.WebAPIConnUpgradeTypeWebSocket):
		return constants.WebAPIConnUpgradeTypeWebSocket, h.upgradeALPN, nil
	case slices.Contains(upgrades, constants.WebAPIConnUpgradeTypeALPNPing):
		return constants.WebAPIConnUpgradeTypeALPNPing, h.upgradeALPNWithPing, nil
	case slices.Contains(upgrades, constants.WebAPIConnUpgradeTypeALPN):
		return constants.WebAPIConnUpgradeTypeALPN, h.upgradeALPN, nil
	default:
		return "", nil, trace.NotFound("unsupported upgrade types: %v", upgrades)
	}
}

// connectionUpgrade handles connection upgrades.
func (h *Handler) connectionUpgrade(w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error) {
	upgradeType, upgradeHandler, err := h.selectConnectionUpgrade(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if upgradeType == constants.WebAPIConnUpgradeTypeWebSocket {
		return h.upgradeALPNWebSocket(w, r, upgradeHandler)
	}

	// TODO(greedy52) DELETE legacy upgrade in 19.0. Client side is deprecated
	// in 18.0.
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, trace.BadParameter("failed to hijack connection")
	}

	conn, _, err := hj.Hijack()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer conn.Close()

	// Since w is hijacked, there is no point returning an error for response
	// starting at this point.
	if err := writeUpgradeResponse(conn, upgradeType); err != nil {
		h.logger.ErrorContext(r.Context(), "Failed to write upgrade response.", "error", err)
		return nil, nil
	}

	if err := upgradeHandler(r.Context(), conn); err != nil && !utils.IsOKNetworkError(err) {
		h.logger.ErrorContext(r.Context(), "Failed to handle upgrade request.", "type", upgradeType, "error", err)
	}
	return nil, nil
}

func (h *Handler) upgradeALPNWebSocket(w http.ResponseWriter, r *http.Request, upgradeHandler ConnectionHandler) (any, error) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
		Subprotocols: []string{
			constants.WebAPIConnUpgradeTypeALPN,
			constants.WebAPIConnUpgradeTypeALPNPing,
		},
	}
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.DebugContext(r.Context(), "Failed to upgrade WebSocket.", "error", err)
		return nil, trace.Wrap(err)
	}
	defer wsConn.Close()

	h.logger.Log(r.Context(), logutils.TraceLevel, "Received WebSocket upgrade.", "protocol", wsConn.Subprotocol())

	// websocketALPNServerConn uses "github.com/gobwas/ws" on the raw net.Conn
	// instead of gorilla's websocket.Conn to workaround an issue that
	// websocket.Conn caches read error when websocketALPNServerConn is passed
	// to a HTTP server and get hijacked for another upgrade. Note that client
	// side's (api/client) websocket ALPN connection wrapper also uses
	// "github.com/gobwas/ws".
	conn := newWebSocketALPNServerConn(r.Context(), wsConn.NetConn(), h.logger)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	switch wsConn.Subprotocol() {
	case constants.WebAPIConnUpgradeTypeALPNPing:
		// Starts native WebSocket ping for "alpn-ping".
		go h.startPing(ctx, conn)
	case constants.WebAPIConnUpgradeTypeALPN:
		// Nothing to do
	default:
		// Just close the connection. Upgrader hijacks the connection so no
		// point returning an error.
		h.logger.DebugContext(ctx, "Unknown or empty WebSocket subprotocol.", "client_protocols", websocket.Subprotocols(r))
		return nil, nil
	}

	if err := upgradeHandler(ctx, conn); err != nil && !utils.IsOKNetworkError(err) {
		// Upgrader hijacks the connection so no point returning an error here.
		h.logger.ErrorContext(ctx, "Failed to handle WebSocket upgrade request",
			"protocol", wsConn.Subprotocol(),
			"error", err,
			"remote_addr", logutils.StringerAttr(conn.RemoteAddr()),
		)
	}
	return nil, nil
}

func (h *Handler) upgradeALPN(ctx context.Context, conn net.Conn) error {
	if h.cfg.ALPNHandler == nil {
		return trace.BadParameter("missing ALPNHandler")
	}

	// ALPNHandler may handle some connections asynchronously. Here we want to
	// block until the handling is done by waiting until the connection is
	// closed.
	waitConn := newWaitConn(ctx, conn)
	defer waitConn.WaitForClose()

	return h.cfg.ALPNHandler(ctx, waitConn)
}

func (h *Handler) upgradeALPNWithPing(ctx context.Context, conn net.Conn) error {
	if h.cfg.ALPNHandler == nil {
		return trace.BadParameter("missing ALPNHandler")
	}

	pingConn := pingconn.New(conn)

	// Cancel ping background goroutine when connection is closed.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go h.startPing(ctx, pingConn)

	return h.upgradeALPN(ctx, pingConn)
}

type pingWriter interface {
	WritePing() error
}

func (h *Handler) startPing(ctx context.Context, pingConn pingWriter) {
	ticker := time.NewTicker(defaults.ProxyPingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := pingConn.WritePing()
			if err != nil {
				if !utils.IsOKNetworkError(err) {
					h.logger.WarnContext(ctx, "Failed to write ping message.", "error", err)
				}
				return
			}
		}
	}
}

func writeUpgradeResponse(w io.Writer, upgradeType string) error {
	header := make(http.Header)
	header.Add(constants.WebAPIConnUpgradeHeader, upgradeType)
	header.Add(constants.WebAPIConnUpgradeTeleportHeader, upgradeType)
	header.Add(constants.WebAPIConnUpgradeConnectionHeader, constants.WebAPIConnUpgradeConnectionType)
	response := &http.Response{
		Status:     http.StatusText(http.StatusSwitchingProtocols),
		StatusCode: http.StatusSwitchingProtocols,
		Header:     header,
		ProtoMajor: 1,
		ProtoMinor: 1,
	}
	return response.Write(w)
}

// waitConn is a net.Conn that provides a "WaitForClose" function to wait until
// the connection is closed.
type waitConn struct {
	net.Conn
	ctx    context.Context
	cancel context.CancelFunc
}

// newWaitConn creates a new waitConn.
func newWaitConn(ctx context.Context, conn net.Conn) *waitConn {
	ctx, cancel := context.WithCancel(ctx)
	return &waitConn{
		Conn:   conn,
		ctx:    ctx,
		cancel: cancel,
	}
}

// WaitForClose blocks until the Close() function of this connection is called.
func (conn *waitConn) WaitForClose() {
	<-conn.ctx.Done()
}

// Close implements net.Conn.
func (conn *waitConn) Close() error {
	err := conn.Conn.Close()
	conn.cancel()
	return trace.Wrap(err)
}

func (conn *waitConn) NetConn() net.Conn {
	return conn.Conn
}

type websocketALPNServerConn struct {
	net.Conn
	readBuffer []byte
	readError  error
	readMutex  sync.Mutex
	writeMutex sync.Mutex

	logContext context.Context
	logger     *slog.Logger
}

func newWebSocketALPNServerConn(ctx context.Context, conn net.Conn, logger *slog.Logger) *websocketALPNServerConn {
	return &websocketALPNServerConn{
		Conn:       conn,
		logContext: ctx,
		logger:     logger.With(teleport.ComponentKey, teleport.Component(teleport.ComponentWeb, "alpnws")),
	}
}

func (c *websocketALPNServerConn) NetConn() net.Conn {
	return c.Conn
}

func (c *websocketALPNServerConn) Read(b []byte) (int, error) {
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
		c.logger.Log(c.logContext, logutils.TraceLevel, "Cleared cached read error.", "err", netError)
		return n, netError
	}
	return n, trace.Wrap(err)
}

func (c *websocketALPNServerConn) readLocked(b []byte) (int, error) {
	// Stop reading if any previous read err.
	if c.readError != nil {
		return 0, trace.Wrap(c.readError)
	}

	if len(c.readBuffer) > 0 {
		n := copy(b, c.readBuffer)
		if n < len(c.readBuffer) {
			c.readBuffer = c.readBuffer[n:]
		} else {
			c.readBuffer = nil
		}
		return n, nil
	}

	for {
		frame, err := ws.ReadFrame(c.Conn)
		if err != nil {
			c.readError = err
			return 0, trace.Wrap(err)
		}

		// All client frames should be masked.
		if frame.Header.Masked {
			frame = ws.UnmaskFrame(frame)
		}

		c.logger.Log(c.logContext, logutils.TraceLevel, "Read websocket frame.", "op", frame.Header.OpCode, "payload_len", len(frame.Payload))

		switch frame.Header.OpCode {
		case ws.OpClose:
			return 0, io.EOF
		case ws.OpBinary:
			c.readBuffer = frame.Payload
			return c.readLocked(b)
		case ws.OpPong:
			// Receives Pong as response to Ping. Nothing to do.
		}
	}
}

func (c *websocketALPNServerConn) writeFrame(frame ws.Frame) error {
	c.logger.Log(c.logContext, logutils.TraceLevel, "Writing websocket frame.", "op", frame.Header.OpCode, "payload_len", len(frame.Payload))

	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()

	// There is no need to mask from server to client.
	return trace.Wrap(ws.WriteFrame(c.Conn, frame))
}

func (c *websocketALPNServerConn) Write(b []byte) (n int, err error) {
	binaryFrame := ws.NewBinaryFrame(b)
	if err := c.writeFrame(binaryFrame); err != nil {
		return 0, trace.Wrap(err)
	}
	return len(b), nil
}

func (c *websocketALPNServerConn) WritePing() error {
	pingFrame := ws.NewPingFrame([]byte(teleport.ComponentTeleport))
	return trace.Wrap(c.writeFrame(pingFrame))
}

func (c *websocketALPNServerConn) SetDeadline(t time.Time) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	return trace.Wrap(c.Conn.SetDeadline(t))
}

func (c *websocketALPNServerConn) SetWriteDeadline(t time.Time) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	return trace.Wrap(c.Conn.SetWriteDeadline(t))
}

func (c *websocketALPNServerConn) SetReadDeadline(t time.Time) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	return trace.Wrap(c.Conn.SetReadDeadline(t))
}
