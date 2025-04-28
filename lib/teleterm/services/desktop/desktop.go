// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package desktop

import (
	"context"
	"net"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/proxy"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
	"github.com/gravitational/teleport/lib/utils"
)

// Connect starts a remote desktop session.
func Connect(ctx context.Context, stream grpc.BidiStreamingServer[api.ConnectToDesktopRequest, api.ConnectToDesktopResponse], clusterClient *client.TeleportClient, proxyClient *proxy.Client, desktopName, login string) error {
	keyRing, err := clusterClient.IssueUserCertsWithMFA(ctx, client.ReissueParams{
		RouteToCluster: clusterClient.SiteName,
		TTL:            clusterClient.KeyTTL,
		RouteToWindowsDesktop: proto.RouteToWindowsDesktop{
			WindowsDesktop: desktopName,
			Login:          login,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	cert, err := keyRing.WindowsDesktopTLSCert(desktopName)
	if err != nil {
		return trace.Wrap(err)
	}

	tlsConfig, err := clusterClient.LoadTLSConfig()
	if err != nil {
		return trace.Wrap(err)
	}

	conn, err := proxyClient.ProxyWindowsDesktopSession(ctx, clusterClient.SiteName, desktopName, cert, tlsConfig.RootCAs)
	if err != nil {
		return trace.Wrap(err)
	}
	defer conn.Close()

	tdpConn := tdp.NewConn(conn)
	defer tdpConn.Close()

	// Now that we have a connection to the desktop service, we can
	// send the username.
	err = tdpConn.WriteMessage(tdp.ClientUsername{Username: login})
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(proxyTdpConn(conn, tdpConn, stream))
}

// proxyTdpConn proxies messages between upstream tdp connection and downstream bidi stream.
func proxyTdpConn(
	upstreamConn net.Conn,
	upstreamConnTdp *tdp.Conn,
	downstreamConn grpc.BidiStreamingServer[api.ConnectToDesktopRequest, api.ConnectToDesktopResponse],
) error {
	errCh := make(chan error, 2)

	// Upstream → Downstream (tdp.Conn → gRPC)
	go func() {
		for {
			// We avoid using io.Copy here, as we want to make sure
			// each TDP message is sent as a unit so that a single
			// 'message' event is emitted in the JS TDP client.
			// Internal buffer of io.Copy could split one message
			// into multiple downstreamConn.Send() calls.
			readMsg, err := upstreamConnTdp.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}

			encoded, err := readMsg.Encode()
			if err != nil {
				errCh <- err
				return
			}

			msg := &api.ConnectToDesktopResponse{Data: encoded}
			if err := downstreamConn.Send(msg); err != nil {
				errCh <- err
				return
			}
		}
	}()

	// Downstream → Upstream (gRPC → net.Conn)
	go func() {
		for {
			resp, err := downstreamConn.Recv()
			switch {
			case utils.IsOKNetworkError(err):
				errCh <- nil
				return
			case err != nil:
				errCh <- err
				return
			}

			_, err = upstreamConn.Write(resp.Data)
			if err != nil {
				errCh <- err
				return
			}
		}
	}()

	// Wait for one side to finish
	return <-errCh
}
