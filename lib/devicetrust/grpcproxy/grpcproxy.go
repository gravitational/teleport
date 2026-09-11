// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package grpcproxy

import (
	"context"
	"log/slog"
	"net"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/peer"

	publicdevicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/public/v1"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/devicetrust/grpcproxy/clientaddr"
	grpcutils "github.com/gravitational/teleport/lib/utils/grpc"
)

// AuthClient is a subset of the full Auth API that must be connected.
type AuthClient interface {
	PublicDevicesClient() publicdevicepb.DeviceTrustServiceClient
}

// ServiceConfig is the configuration for [New].
type ServiceConfig struct {
	AuthClient AuthClient
	Log        *slog.Logger
}

// New creates a new [Service].
func New(cfg ServiceConfig) (*Service, error) {
	if cfg.AuthClient == nil {
		return nil, trace.BadParameter("missing AuthClient")
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}

	return &Service{
		authClient: cfg.AuthClient,
		log:        cfg.Log,
	}, nil
}

// Service proxies requests from the public service in the Proxy Service to the
// equivalent service in the Auth Service.
type Service struct {
	publicdevicepb.UnimplementedDeviceTrustServiceServer
	authClient AuthClient
	log        *slog.Logger
}

// CreatePairedDeviceEnrollToken forwards the request to the same RPC in the
// Auth Service.
func (s *Service) CreatePairedDeviceEnrollToken(ctx context.Context, req *publicdevicepb.CreatePairedDeviceEnrollTokenRequest) (*publicdevicepb.CreatePairedDeviceEnrollTokenResponse, error) {
	ctx, err := clientAddrContext(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := s.authClient.PublicDevicesClient().CreatePairedDeviceEnrollToken(ctx, req)
	return res, trace.Wrap(err)
}

// errEnrollDeviceFirstMessageTimeout ends a stream whose client sent no init
// message within [devicetrust.PublicEnrollDeviceFirstMessageTimeout]. It is a
// LimitExceededError because the caller ran into a limit the server imposes on
// the stream, rather than just a regular exceeded deadline.
var errEnrollDeviceFirstMessageTimeout = &trace.LimitExceededError{Message: "device enrollment timed out waiting for the init message"}

// errEnrollDeviceProxyTimeout ends a stream that outlived
// [devicetrust.PublicEnrollDeviceProxyTimeout]. See
// [errEnrollDeviceFirstMessageTimeout] for the choice of error type.
var errEnrollDeviceProxyTimeout = &trace.LimitExceededError{Message: "device enrollment timed out on the proxy"}

// EnrollDevice forwards the enrollment ceremony stream to the same RPC in the
// Auth Service.
func (s *Service) EnrollDevice(stream publicdevicepb.DeviceTrustService_EnrollDeviceServer) error {
	err := grpcutils.ProxyBidiStream(s.log, stream,
		func(ctx context.Context) (publicdevicepb.DeviceTrustService_EnrollDeviceClient, error) {
			ctx, err := clientAddrContext(ctx)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			server, err := s.authClient.PublicDevicesClient().EnrollDevice(ctx)
			return server, trace.Wrap(err)
		},
		grpcutils.WithFirstClientMessageTimeout(devicetrust.PublicEnrollDeviceFirstMessageTimeout, errEnrollDeviceFirstMessageTimeout),
		grpcutils.WithStreamTimeout(devicetrust.PublicEnrollDeviceProxyTimeout, errEnrollDeviceProxyTimeout),
	)
	return trace.Wrap(err)
}

// clientAddrContext returns ctx with the address of the calling client set as
// the forwarded client address for the Auth Service. See [clientaddr.Header].
func clientAddrContext(ctx context.Context) (context.Context, error) {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return nil, trace.Errorf("client address unavailable")
	}
	tcpAddr, ok := p.Addr.(*net.TCPAddr)
	if !ok {
		return nil, trace.Errorf("client address %v (%T) is not a TCP address", p.Addr, p.Addr)
	}
	return clientaddr.WithOutgoingContext(ctx, tcpAddr), nil
}
