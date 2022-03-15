// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"net"
	"strings"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// proxyService impelements the grpc ProxyService.
type proxyService struct {
	clusterDialer ClusterDialer
	log           logrus.FieldLogger
}

// DialNode opens a bidrectional stream to the requested node.
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
		return trace.BadParameter("invalid dial request: source and destinatation must not be nil")
	}

	log := s.log.WithFields(logrus.Fields{
		"node": dial.NodeID,
		"src":  dial.Source.Addr,
		"dst":  dial.Destination.Addr,
	})
	log.Debugf("Dial request from peer.")

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

	nodeConn, err := s.clusterDialer.Dial(clusterName, reversetunnel.DialParams{
		From:     source,
		To:       destination,
		ServerID: dial.NodeID,
		ConnType: dial.TunnelType,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = stream.Send(&proto.Frame{
		Message: &proto.Frame_ConnectionEstablished{},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	streamConn := newStreamConn(stream, source, destination)
	go func() {
		err := streamConn.run()
		log.WithError(err).Debug("Stream connection exited.")
	}()

	sent, received, err := pipeConn(stream.Context(), streamConn, nodeConn)
	log.Debugf("Closing dial request from peer. sent: %d reveived %d", sent, received)
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
	Dial(clusterName string, request reversetunnel.DialParams) (net.Conn, error)
}

// clusterDialerFunc is a function that implements ClusterDialer.
type clusterDialerFunc func(clusterName string, request reversetunnel.DialParams) (net.Conn, error)

func (f clusterDialerFunc) Dial(clusterName string, request reversetunnel.DialParams) (net.Conn, error) {
	return f(clusterName, request)
}

// NewClusterDialer implements ClusterDialer for a reverse tunnel server.
func NewClusterDialer(server reversetunnel.Server) ClusterDialer {
	return clusterDialerFunc(func(clusterName string, request reversetunnel.DialParams) (net.Conn, error) {
		site, err := server.GetSite(clusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		conn, err := site.Dial(request)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return conn, nil
	})
}
