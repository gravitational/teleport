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
