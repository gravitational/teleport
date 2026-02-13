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
	"cmp"
	"log/slog"
	"net/http"

	"github.com/gobwas/ws"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
)

var wsProtocolUpgrader = ws.HTTPUpgrader{
	Protocol: func(clientProtocol string) bool {
		// Accepts any protocol, but we only use ALPN ping and ALPN.
		return clientProtocol == constants.WebAPIConnUpgradeTypeALPNPing ||
			clientProtocol == constants.WebAPIConnUpgradeTypeALPN ||
			clientProtocol == constants.WebAPIConnUpgradeProtocolWebSocketClose
	},
}

// NewServerConnection creates a new WebsocketUpgradeConn for the server side.
// It upgrades the HTTP request to a WebSocket connection and returns the
// WebsocketUpgradeConn instance. The connection is configured for server-side
// use, allowing it to handle WebSocket messages and pings.
func NewServerConnection(
	logger *slog.Logger,
	r *http.Request,
	w http.ResponseWriter,
) (*Conn, error) {
	// Upgrade the HTTP request to a WebSocket connection.
	conn, _, hs, err := wsProtocolUpgrader.Upgrade(r, w)
	if err != nil {
		if conn != nil {
			conn.Close()
		}
		return nil, trace.Wrap(err, "failed to upgrade WebSocket connection")
	}
	if hs.Protocol == "" {
		// Write a close frame with an unsupported protocol error.
		_ = ws.WriteFrame(conn,
			ws.NewCloseFrame(
				ws.NewCloseFrameBody(
					ws.StatusUnsupportedData,
					"unsupported WebSocket sub-protocol: unsupported-protocol",
				),
			),
		)
		_ = conn.Close()
		return nil, trace.BadParameter("unsupported WebSocket sub-protocol: %q", hs.Protocol)
	}

	return newWebsocketUpgradeConn(newWebsocketUpgradeConnConfig{
		ctx:      r.Context(),
		conn:     conn,
		logger:   cmp.Or(logger, slog.Default()),
		hs:       hs,
		connType: serverConnection,
	}), nil
}
