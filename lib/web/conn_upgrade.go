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
	"time"

	"github.com/gobwas/ws"
	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	"github.com/gravitational/teleport/api/client/websocketupgradeproto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// connectionUpgrade handles connection upgrades.
func (h *Handler) connectionUpgrade(w http.ResponseWriter, r *http.Request, p httprouter.Params) (any, error) {
	upgrades := r.Header.Values(constants.WebAPIConnUpgradeHeader)
	if !slices.Contains(upgrades, constants.WebAPIConnUpgradeTypeWebSocket) {
		return nil, trace.NotFound("unsupported upgrade types: %v", upgrades)
	}

	return h.upgradeALPNWebSocket(w, r, h.upgradeALPN)
}

func (h *Handler) upgradeALPNWebSocket(w http.ResponseWriter, r *http.Request, upgradeHandler ConnectionHandler) (any, error) {
	conn, err := h.websocketUpgrade(r, w)
	if err != nil {
		h.logger.DebugContext(r.Context(), "Failed to upgrade WebSocket.", "error", err)
		return nil, trace.Wrap(err)
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	h.logger.Log(r.Context(), logutils.TraceLevel, "Received WebSocket upgrade.", "protocol", conn.Protocol())

	switch conn.Protocol() {
	case constants.WebAPIConnUpgradeTypeALPNPing, constants.WebAPIConnUpgradeProtocolWebSocketClose:
		// Starts native WebSocket ping for "alpn-ping".
		go h.startPing(ctx, conn)
	case constants.WebAPIConnUpgradeTypeALPN:
		// Nothing to do
	default:
		// Just close the connection. Upgrader hijacks the connection so no
		// point returning an error.
		h.logger.DebugContext(ctx, "Unknown or empty WebSocket subprotocol.", "client_protocols", websocket.Subprotocols(r))
		conn.CloseWithStatus(ws.StatusUnsupportedData, "unknown or empty subprotocol")
		return nil, nil
	}

	if err := upgradeHandler(ctx, conn); err != nil && !utils.IsOKNetworkError(err) {
		// Upgrader hijacks the connection so no point returning an error here.
		h.logger.ErrorContext(ctx, "Failed to handle WebSocket upgrade request",
			"protocol", conn.Protocol(),
			"error", err,
			"remote_addr", logutils.StringerAttr(conn.RemoteAddr()),
		)
	}
	return nil, nil
}

// websocketUpgrade upgrades the HTTP request to a WebSocket connection and
// returns the WebSocket connection and the handshake information.
func (h *Handler) websocketUpgrade(r *http.Request, w http.ResponseWriter) (*websocketupgradeproto.Conn, error) {
	conn, err := websocketupgradeproto.NewServerConnection(
		h.logger.With("component", websocketupgradeproto.ComponentTeleport),
		r, w,
	)
	return conn, trace.Wrap(err, "failed to upgrade WebSocket connection")
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
// This function cancels the context to unblock WaitForClose but
// it should never close the underlying connection. This occurs because the
// connection is a websocket connection that requires a graceful close
// with a close frame.
func (conn *waitConn) Close() error {
	conn.cancel()
	return nil
}

func (conn *waitConn) NetConn() net.Conn {
	return conn.Conn
}
