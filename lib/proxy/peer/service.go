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
	"log/slog"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	streamutils "github.com/gravitational/teleport/api/utils/grpc/stream"
	peerdial "github.com/gravitational/teleport/lib/proxy/peer/dial"
	"github.com/gravitational/teleport/lib/utils"
)

// proxyService implements the grpc ProxyService.
type proxyService struct {
	dialer peerdial.Dialer
	log    *slog.Logger
}

// DialNode opens a bidirectional stream to the requested node.
func (s *proxyService) DialNode(stream proto.ProxyService_DialNodeServer) error {
	frame, err := stream.Recv()
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

	log := s.log.With(
		"node", dial.NodeID,
		"src", dial.Source.Addr,
		"dst", dial.Destination.Addr,
	)
	log.DebugContext(stream.Context(), "dial request from peer")

	_, clusterName, err := splitServerID(dial.NodeID)
	if err != nil {
		return trace.Wrap(err)
	}

	source := &utils.NetAddr{
		Addr:        dial.Source.Addr,
		AddrNetwork: dial.Source.Network,
	}
	destination := &utils.NetAddr{
		Addr:        dial.Destination.Addr,
		AddrNetwork: dial.Destination.Network,
	}

	nodeConn, err := s.dialer.Dial(clusterName, peerdial.DialParams{
		From:     source,
		To:       destination,
		ServerID: dial.NodeID,
		ConnType: dial.TunnelType,
		Permit:   dial.GetPermit(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = stream.Send(&proto.Frame{
		Message: &proto.Frame_ConnectionEstablished{
			ConnectionEstablished: &proto.ConnectionEstablished{},
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	streamRW, err := streamutils.NewReadWriter(frameStream{stream: stream})
	if err != nil {
		return trace.Wrap(err)
	}

	streamConn := utils.NewTrackingConn(streamutils.NewConn(streamRW, source, destination))

	err = utils.ProxyConn(stream.Context(), streamConn, nodeConn)
	sent, received := streamConn.Stat()
	log.DebugContext(stream.Context(), "closing dial request from peer", "sent", sent, "received", received)
	return trace.Wrap(err)
}

func (s *proxyService) Ping(ctx context.Context, _ *proto.ProxyServicePingRequest) (*proto.ProxyServicePingResponse, error) {
	return new(proto.ProxyServicePingResponse), nil
}

// splitServerID splits a server id in to a node id and cluster name.
func splitServerID(address string) (string, string, error) {
	split := strings.Split(address, ".")
	if len(split) == 0 || split[0] == "" {
		return "", "", trace.BadParameter("invalid server id: \"%s\"", address)
	}

	return split[0], strings.Join(split[1:], "."), nil
}
