/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package proxy

import (
	"fmt"
	"net/http"

	gwebsocket "github.com/gorilla/websocket"
	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/util/httpstream"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	kwebsocket "k8s.io/client-go/transport/websocket"

	"github.com/gravitational/teleport/lib/kube/internal"
)

// WebsocketRoundTripper knows how to upgrade an HTTP request to one that supports
// multiplexed streams. After RoundTrip() is invoked, Conn will be set
// and usable. WebsocketRoundTripper implements the UpgradeRoundTripper interface.
type WebsocketRoundTripper struct {
	roundTripperConfig

	// conn is the websocket network connection to the remote server.
	conn *gwebsocket.Conn

	// onConnected is a hook that happens when connection was successfully established,
	// can be used to propagate established connection somewhere else - we are using it
	// to set underlying connection of the native k8s websocket executor.
	onConnected func(conn *gwebsocket.Conn)
}

// NewWebsocketRoundTripperWithDialer creates a new WebsocketRoundTripper that will
// dial and upgrade connection, copying impersonation setup specified in the config.
func NewWebsocketRoundTripperWithDialer(cfg roundTripperConfig) *WebsocketRoundTripper {
	return &WebsocketRoundTripper{roundTripperConfig: cfg}
}

// RoundTrip executes the Request and upgrades it.
func (w *WebsocketRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	header := utilnet.CloneHeader(req.Header)
	// copyImpersonationHeaders copies the headers from the original request to the new
	// request headers. This is necessary to forward the original user's impersonation
	// when multiple kubernetes_users are available.
	copyImpersonationHeaders(header, w.originalHeaders)
	if err := setupImpersonationHeaders(w.sess, header); err != nil {
		return nil, trace.Wrap(err)
	}

	var err error

	// If we're using identity forwarding, we need to add the impersonation
	// headers to the request before we send the request.
	if w.useIdentityForwarding {
		if header, err = internal.IdentityForwardingHeaders(w.ctx, header); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	clone := utilnet.CloneRequest(req)
	clone.Header = header

	nativeBufferSize := (&kwebsocket.RoundTripper{}).DataBufferSize()

	wsDialer := gwebsocket.Dialer{
		NetDialContext:  w.dialWithContext,
		Proxy:           w.proxier,
		TLSClientConfig: w.tlsConfig,
		Subprotocols:    header[httpstream.HeaderProtocolVersion],
		ReadBufferSize:  nativeBufferSize + 1024, // matching code in k8s websocket/roundripper.go
		WriteBufferSize: nativeBufferSize + 1024,
	}

	switch clone.URL.Scheme {
	case "https":
		clone.URL.Scheme = "wss"
	case "http":
		clone.URL.Scheme = "ws"
	default:
		return nil, fmt.Errorf("unknown url scheme: %s", clone.URL.Scheme)
	}

	wsConn, wsResp, err := wsDialer.DialContext(w.ctx, clone.URL.String(), clone.Header)
	if err != nil {
		if wsResp != nil {
			return nil, trace.Wrap(extractKubeAPIStatusFromReq(wsResp))
		}
		return nil, &httpstream.UpgradeFailureError{Cause: err}
	}
	w.conn = wsConn
	if w.onConnected != nil {
		w.onConnected(wsConn)
	}

	return wsResp, nil
}
