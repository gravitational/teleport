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

package peer

import (
	"context"
	"net"
	"strings"

	"connectrpc.com/connect"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	streamutils "github.com/gravitational/teleport/api/utils/grpc/stream"
	peerv0 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/proxy/peer/v0"
	"github.com/gravitational/teleport/gen/proto/go/teleport/lib/proxy/peer/v0/peerv0connect"
	"github.com/gravitational/teleport/lib/utils"
)

// proxyService implements the grpc ProxyService.
type proxyService struct {
	clusterDialer ClusterDialer
	log           logrus.FieldLogger
}

var _ peerv0connect.ProxyServiceHandler = (*proxyService)(nil)

// serverFrameStream wraps a server side stream as a [streamutils.Source].
type serverFrameStream struct {
	stream interface {
		Send(*peerv0.DialNodeResponse) error
		Receive() (*peerv0.DialNodeRequest, error)
	}
}

func (s *serverFrameStream) Send(p []byte) error {
	return trace.Wrap(s.stream.Send(&peerv0.DialNodeResponse{Message: &peerv0.DialNodeResponse_Data{Data: &peerv0.Data{Bytes: p}}}))
}

func (s *serverFrameStream) Recv() ([]byte, error) {
	frame, err := s.stream.Receive()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	data := frame.GetData()
	if data == nil {
		return nil, trace.BadParameter("received invalid frame")
	}

	return data.GetBytes(), nil
}

// DialNode opens a bidirectional stream to the requested node.
func (s *proxyService) DialNode(ctx context.Context, stream *connect.BidiStream[peerv0.DialNodeRequest, peerv0.DialNodeResponse]) error {
	frame, err := stream.Receive()
	if err != nil {
		return trace.Wrap(err)
	}

	// The first frame is always expected to be a dial request.
	dial := frame.GetDialRequest()
	if dial == nil {
		return trace.BadParameter("invalid dial request: request must not be nil")
	}

	if dial.Source == nil || dial.Destination == nil {
		return trace.BadParameter("invalid dial request: source and destination must not be nil")
	}

	log := s.log.WithFields(logrus.Fields{
		"node": dial.NodeId,
		"src":  dial.Source.Address,
		"dst":  dial.Destination.Address,
	})
	log.Debugf("Dial request from peer.")

	_, clusterName, err := splitServerID(dial.NodeId)
	if err != nil {
		return trace.Wrap(err)
	}

	source := &utils.NetAddr{
		Addr:        dial.Source.Address,
		AddrNetwork: dial.Source.Network,
	}
	destination := &utils.NetAddr{
		Addr:        dial.Destination.Address,
		AddrNetwork: dial.Destination.Network,
	}

	nodeConn, err := s.clusterDialer.Dial(clusterName, DialParams{
		From:     source,
		To:       destination,
		ServerID: dial.NodeId,
		ConnType: types.TunnelType(dial.TunnelType),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = stream.Send(&peerv0.DialNodeResponse{
		Message: &peerv0.DialNodeResponse_ConnectionEstablished{
			ConnectionEstablished: &peerv0.ConnectionEstablished{},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	streamRW, err := streamutils.NewReadWriter(&serverFrameStream{stream: stream})
	if err != nil {
		return trace.Wrap(err)
	}

	streamConn := utils.NewTrackingConn(streamutils.NewConn(streamRW, source, destination))

	err = utils.ProxyConn(ctx, streamConn, nodeConn)
	sent, received := streamConn.Stat()
	log.Debugf("Closing dial request from peer. sent: %d received %d", sent, received)
	return trace.Wrap(err)
}

// splitServerID splits a server id in to a node id and cluster name.
func splitServerID(address string) (string, string, error) {
	split := strings.Split(address, ".")
	if len(split) == 0 || split[0] == "" {
		return "", "", trace.BadParameter("invalid server id: \"%s\"", address)
	}

	return split[0], strings.Join(split[1:], "."), nil
}

// ClusterDialer dials a node in the given cluster.
type ClusterDialer interface {
	Dial(clusterName string, request DialParams) (net.Conn, error)
}

type DialParams struct {
	From     *utils.NetAddr
	To       *utils.NetAddr
	ServerID string
	ConnType types.TunnelType
}
