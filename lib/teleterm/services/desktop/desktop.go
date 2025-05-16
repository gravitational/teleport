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

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/proxy"
	streamutils "github.com/gravitational/teleport/api/utils/grpc/stream"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/desktop/tdp"
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

	// Now that we have a connection to the desktop service, we can
	// send the username.
	tdpConn := tdp.NewConn(conn)
	defer tdpConn.Close()
	err = tdpConn.WriteMessage(tdp.ClientUsername{Username: login})
	if err != nil {
		return trace.Wrap(err)
	}

	downstreamRW, err := streamutils.NewReadWriter(&clientStream{
		stream: stream,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	tdpConnProxy := tdp.NewConnProxy(downstreamRW, conn, nil)
	return trace.Wrap(tdpConnProxy.Run())
}

// clientStream implements the [streamutils.Source] interface
// for a [teletermv1.TerminalService_ConnectToDesktopClient].
type clientStream struct {
	stream grpc.BidiStreamingServer[api.ConnectToDesktopRequest, api.ConnectToDesktopResponse]
}

func (d clientStream) Send(p []byte) error {
	return trace.Wrap(d.stream.Send(&api.ConnectToDesktopResponse{Data: p}))
}

func (d clientStream) Recv() ([]byte, error) {
	msg, err := d.stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if msg.GetTargetDesktop().GetDesktopUri() != "" || msg.GetTargetDesktop().GetLogin() != "" {
		return nil, trace.BadParameter("target desktop can be send only in the first message")
	}

	data := msg.GetData()
	if data == nil {
		return nil, trace.BadParameter("received invalid message")
	}

	return data, nil
}
