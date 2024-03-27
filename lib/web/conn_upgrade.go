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
	"io"
	"net"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/utils/pingconn"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
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
func (h *Handler) connectionUpgrade(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	upgradeType, upgradeHandler, err := h.selectConnectionUpgrade(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if upgradeType == constants.WebAPIConnUpgradeTypeWebSocket {
		return h.upgradeALPNWebSocket(w, r, upgradeHandler)
	}

	// TODO(greedy52) DELETE legacy upgrade in 17.0.
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
		h.log.WithError(err).Error("Failed to write upgrade response.")
		return nil, nil
	}

	if err := upgradeHandler(r.Context(), conn); err != nil && !utils.IsOKNetworkError(err) {
		h.log.WithError(err).Errorf("Failed to handle %v upgrade request.", upgradeType)
	}
	return nil, nil
}

func (h *Handler) upgradeALPNWebSocket(w http.ResponseWriter, r *http.Request, upgradeHandler ConnectionHandler) (interface{}, error) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
		Subprotocols: []string{
			constants.WebAPIConnUpgradeTypeALPN,
			constants.WebAPIConnUpgradeTypeALPNPing,
		},
	}
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.WithError(err).Debug("Failed to upgrade weboscket.")
		return nil, trace.Wrap(err)
	}
	defer wsConn.Close()

	logrus.WithField("protocol", wsConn.Subprotocol()).Trace("Received WebSocket upgrade.")

	conn := newWebSocketALPNServerConn(wsConn)
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
		h.log.WithField("client-protocols", websocket.Subprotocols(r)).
			Debug("Unknown or empty WebSocket subprotocol.")
		return nil, nil
	}

	if err := upgradeHandler(ctx, conn); err != nil && !utils.IsOKNetworkError(err) {
		// Upgrader hijacks the connection so no point returning an error here.
		h.log.WithError(err).WithField("protocol", wsConn.Subprotocol()).Errorf("Failed to handle WebSocket upgrade request.")
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
					h.log.WithError(err).Warn("Failed to write ping message")
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

type websocketALPNServerConn struct {
	*websocket.Conn
	readBuffer []byte
	readError  error
	readMutex  sync.Mutex
	writeMutex sync.Mutex
}

func newWebSocketALPNServerConn(wsConn *websocket.Conn) *websocketALPNServerConn {
	return &websocketALPNServerConn{
		Conn: wsConn,
	}
}

func (c *websocketALPNServerConn) convertError(err error) error {
	if isOKWebsocketCloseError(err) {
		return io.EOF
	}
	return err
}

func (c *websocketALPNServerConn) Read(b []byte) (int, error) {
	c.readMutex.Lock()
	defer c.readMutex.Unlock()

	n, err := c.readLocked(b)
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
		messageType, data, err := c.Conn.ReadMessage()
		if err != nil {
			c.readError = c.convertError(err)
			return 0, trace.Wrap(c.readError)
		}

		switch messageType {
		case websocket.CloseMessage:
			return 0, nil
		case websocket.BinaryMessage:
			c.readBuffer = data
			return c.readLocked(b)
		case websocket.PongMessage:
			// Receives Pong as response to Ping. Nothing to do.
		}
	}
}

func (c *websocketALPNServerConn) Write(b []byte) (n int, err error) {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	if err := c.Conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return 0, trace.Wrap(c.convertError(err))
	}
	return len(b), nil
}

func (c *websocketALPNServerConn) WritePing() error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()

	// Send some identifier with Ping. Note that we are not validating the Pong
	// response.
	err := c.Conn.WriteMessage(websocket.PingMessage, []byte(teleport.ComponentTeleport))
	return trace.Wrap(c.convertError(err))
}

func (c *websocketALPNServerConn) SetDeadline(t time.Time) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	return trace.NewAggregate(
		c.Conn.SetReadDeadline(t),
		c.Conn.SetWriteDeadline(t),
	)
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
