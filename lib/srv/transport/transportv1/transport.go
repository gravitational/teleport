// Copyright 2023 Gravitational, Inc
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

package transportv1

import (
	"context"
	"net"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	proxyv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/proxy/v1"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
	streamutils "github.com/gravitational/teleport/lib/utils/grpc/stream"
)

// Dialer is the interface that groups basic dialing methods.
type Dialer interface {
	DialSite(ctx context.Context, clusterName string) (net.Conn, error)
	DialHost(ctx context.Context, from net.Addr, host, port, clusterName string, accessChecker services.AccessChecker, agentGetter teleagent.Getter) (net.Conn, error)
}

// ServerConfig holds creation parameters for Service.
type ServerConfig struct {
	FIPS   bool
	Logger logrus.FieldLogger
	Dialer Dialer
}

// CheckAndSetDefaults ensures required parameters are set
// and applies default values for missing optional parameters.
func (c *ServerConfig) CheckAndSetDefaults() error {
	if c.Dialer == nil {
		return trace.BadParameter("parameter Dialer required")
	}

	if c.Logger == nil {
		c.Logger = utils.NewLogger().WithField(trace.Component, "transport")
	}

	return nil
}

// Service implements the teleport.proxy.v1.ProxyService RPC
// service.
type Service struct {
	proxyv1.UnimplementedProxyServiceServer

	cfg ServerConfig
}

func NewService(cfg ServerConfig) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{cfg: cfg}, nil
}

func (s *Service) GetClusterDetails(context.Context, *proxyv1.GetClusterDetailsRequest) (*proxyv1.GetClusterDetailsResponse, error) {
	return &proxyv1.GetClusterDetailsResponse{Details: &proxyv1.ClusterDetails{FipsEnabled: s.cfg.FIPS}}, nil
}

func (s *Service) ProxyCluster(stream proxyv1.ProxyService_ProxyClusterServer) error {
	req, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err, "failed receiving first frame")
	}

	conn, err := s.cfg.Dialer.DialSite(stream.Context(), req.Cluster)
	if err != nil {
		return trace.Wrap(err, "failed dialing cluster %q", req.Cluster)
	}

	// A client may provide a frame with the first message. Since the message is
	// already consumed it won't be copying during the proxying below. In order for
	// the contents to be copied to the connection it needs to be manually written.
	if req.Frame != nil {
		if _, err := conn.Write(req.Frame.Payload); err != nil {
			return trace.Wrap(err, "failed writing payload from first frame")
		}
	}

	streamRW, err := streamutils.NewReadWriter(clusterStream{stream: stream})
	if err != nil {
		return trace.Wrap(err, "failed constructing streamer")
	}

	return trace.Wrap(utils.ProxyConn(stream.Context(), conn, streamRW))
}

// clusterStream implements the [streamutils.Source] interface
// for a [proxyv1.ProxyService_ProxyClusterServer].
type clusterStream struct {
	stream proxyv1.ProxyService_ProxyClusterServer
}

func (c clusterStream) Recv() ([]byte, error) {
	req, err := c.stream.Recv()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Frame == nil {
		return nil, trace.BadParameter("received invalid frame")
	}

	return req.Frame.Payload, nil
}

func (c clusterStream) Send(frame []byte) error {
	return trace.Wrap(c.stream.Send(&proxyv1.ProxyClusterResponse{Frame: &proxyv1.Frame{Payload: frame}}))
}
