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
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/devicetrust"
	"github.com/gravitational/teleport/lib/devicetrust/grpcproxy/clientaddr"
	"github.com/gravitational/teleport/lib/limiter"
	grpcutils "github.com/gravitational/teleport/lib/utils/grpc"
)

// AuthClient is a subset of the full Auth API that must be connected.
type AuthClient interface {
	PublicDevicesClient() publicdevicepb.DeviceTrustServiceClient
}

// RateLimiter is the subset of [limiter.RateLimiter] the service uses.
type RateLimiter interface {
	RegisterRequest(token string) error
}

// ServiceConfig is the configuration for [New].
type ServiceConfig struct {
	AuthClient AuthClient
	Log        *slog.Logger
	// Limiter rate limits requests by client IP. If nil, a limiter with the rates
	// of the other unauthenticated Teleport endpoints is used, see [New].
	Limiter RateLimiter
}

// errRateLimited is returned to a client whose IP is over its rate limit.
var errRateLimited = &trace.LimitExceededError{Message: "rate limit exceeded"}

// New creates a new [Service].
func New(cfg ServiceConfig) (*Service, error) {
	if cfg.AuthClient == nil {
		return nil, trace.BadParameter("missing AuthClient")
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.Limiter == nil {
		// Same rates as the other unauthenticated endpoints: the limiter lib/web
		// puts on challenge generation and createAuthenticateChallengeLimiter in
		// the Auth Service.
		//
		// The service needs a limiter of its own because the one
		// initPublicGRPCServer in lib/service runs on every RPC has no request
		// rates by default, only a connection limit.
		l, err := limiter.NewRateLimiter(limiter.Config{
			Rates: []limiter.Rate{{
				Period:  defaults.LimiterPeriod,
				Average: defaults.LimiterAverage,
				Burst:   defaults.LimiterBurst,
			}},
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		cfg.Limiter = l
	}

	return &Service{
		authClient: cfg.AuthClient,
		log:        cfg.Log,
		limiter:    cfg.Limiter,
	}, nil
}

// Service proxies requests from the public service in the Proxy Service to the
// equivalent service in the Auth Service.
type Service struct {
	publicdevicepb.UnimplementedDeviceTrustServiceServer
	authClient AuthClient
	log        *slog.Logger
	limiter    RateLimiter
}

// errCreatePairedDeviceEnrollTokenProxyTimeout ends a call that outlived
// [devicetrust.PublicCreatePairedDeviceEnrollTokenProxyTimeout]. It is a
// LimitExceededError because the caller ran into a limit the server imposes on
// the stream, rather than just a regular exceeded deadline.
var errCreatePairedDeviceEnrollTokenProxyTimeout = &trace.LimitExceededError{
	Message: "enrollment token request timed out on the proxy",
}

// CreatePairedDeviceEnrollToken forwards the request to the same RPC in the
// Auth Service.
func (s *Service) CreatePairedDeviceEnrollToken(ctx context.Context, req *publicdevicepb.CreatePairedDeviceEnrollTokenRequest) (*publicdevicepb.CreatePairedDeviceEnrollTokenResponse, error) {
	addr, err := peerTCPAddr(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := s.rateLimitByClientIP(addr); err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := grpcutils.ProxyUnaryCall(ctx, s.log,
		func(ctx context.Context) (*publicdevicepb.CreatePairedDeviceEnrollTokenResponse, error) {
			ctx = clientaddr.WithOutgoingContext(ctx, addr)
			return s.authClient.PublicDevicesClient().CreatePairedDeviceEnrollToken(ctx, req)
		},
		grpcutils.WithCallTimeout(devicetrust.PublicCreatePairedDeviceEnrollTokenProxyTimeout, errCreatePairedDeviceEnrollTokenProxyTimeout),
	)
	return res, trace.Wrap(err)
}

// errEnrollDeviceFirstMessageTimeout ends a stream whose client sent no init
// message within [devicetrust.PublicEnrollDeviceFirstMessageTimeout]. It is a
// LimitExceededError because the caller ran into a limit the server imposes on
// the stream, rather than just a regular exceeded deadline.
var errEnrollDeviceFirstMessageTimeout = &trace.LimitExceededError{
	Message: "device enrollment timed out waiting for the init message",
}

// errEnrollDeviceProxyTimeout ends a stream that outlived
// [devicetrust.PublicEnrollDeviceProxyTimeout]. See
// [errEnrollDeviceFirstMessageTimeout] for the choice of error type.
var errEnrollDeviceProxyTimeout = &trace.LimitExceededError{
	Message: "device enrollment timed out on the proxy",
}

// EnrollDevice forwards the enrollment ceremony stream to the same RPC in the
// Auth Service.
func (s *Service) EnrollDevice(stream publicdevicepb.DeviceTrustService_EnrollDeviceServer) error {
	addr, err := peerTCPAddr(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := s.rateLimitByClientIP(addr); err != nil {
		return trace.Wrap(err)
	}
	err = grpcutils.ProxyBidiStream(s.log, stream,
		func(ctx context.Context) (publicdevicepb.DeviceTrustService_EnrollDeviceClient, error) {
			ctx = clientaddr.WithOutgoingContext(ctx, addr)
			server, err := s.authClient.PublicDevicesClient().EnrollDevice(ctx)
			return server, trace.Wrap(err)
		},
		grpcutils.WithFirstClientMessageTimeout(devicetrust.PublicEnrollDeviceFirstMessageTimeout, errEnrollDeviceFirstMessageTimeout),
		grpcutils.WithStreamTimeout(devicetrust.PublicEnrollDeviceProxyTimeout, errEnrollDeviceProxyTimeout),
	)
	return trace.Wrap(err)
}

// errAuthenticateDeviceFirstMessageTimeout ends a stream whose client sent no
// init message within [devicetrust.PublicAuthenticateDeviceFirstMessageTimeout].
// It is a LimitExceededError because the caller ran into a limit the server
// imposes on the stream, rather than just a regular exceeded deadline.
var errAuthenticateDeviceFirstMessageTimeout = &trace.LimitExceededError{
	Message: "device authentication timed out waiting for the init message",
}

// errAuthenticateDeviceProxyTimeout ends a stream that outlived
// [devicetrust.PublicAuthenticateDeviceProxyTimeout]. See
// [errAuthenticateDeviceFirstMessageTimeout] for the choice of error type.
var errAuthenticateDeviceProxyTimeout = &trace.LimitExceededError{
	Message: "device authentication timed out on the proxy",
}

// AuthenticateDevice forwards the authentication ceremony stream to the same
// RPC in the Auth Service.
func (s *Service) AuthenticateDevice(stream publicdevicepb.DeviceTrustService_AuthenticateDeviceServer) error {
	addr, err := peerTCPAddr(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := s.rateLimitByClientIP(addr); err != nil {
		return trace.Wrap(err)
	}
	err = grpcutils.ProxyBidiStream(s.log, stream,
		func(ctx context.Context) (publicdevicepb.DeviceTrustService_AuthenticateDeviceClient, error) {
			ctx = clientaddr.WithOutgoingContext(ctx, addr)
			server, err := s.authClient.PublicDevicesClient().AuthenticateDevice(ctx)
			return server, trace.Wrap(err)
		},
		grpcutils.WithFirstClientMessageTimeout(devicetrust.PublicAuthenticateDeviceFirstMessageTimeout, errAuthenticateDeviceFirstMessageTimeout),
		grpcutils.WithStreamTimeout(devicetrust.PublicAuthenticateDeviceProxyTimeout, errAuthenticateDeviceProxyTimeout),
	)
	return trace.Wrap(err)
}

func (s *Service) rateLimitByClientIP(addr *net.TCPAddr) error {
	if err := s.limiter.RegisterRequest(addr.IP.String()); err != nil {
		return trace.Wrap(errRateLimited)
	}
	return nil
}

// peerTCPAddr returns the address of the calling client, which the RPCs forward
// to the Auth Service as the client address. See [clientaddr.Header].
func peerTCPAddr(ctx context.Context) (*net.TCPAddr, error) {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return nil, trace.Errorf("client address unavailable")
	}
	tcpAddr, ok := p.Addr.(*net.TCPAddr)
	if !ok {
		return nil, trace.Errorf("client address %v (%T) is not a TCP address", p.Addr, p.Addr)
	}
	return tcpAddr, nil
}
