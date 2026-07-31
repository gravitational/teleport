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

	"github.com/gravitational/trace"
	"google.golang.org/grpc/peer"

	publicdevicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/public/v1"
	"github.com/gravitational/teleport/lib/devicetrust"
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
	ctx, err := forwardClientSrcAddr(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	res, err := s.authClient.PublicDevicesClient().CreatePairedDeviceEnrollToken(ctx, req)
	return res, trace.Wrap(err)
}

// forwardClientSrcAddr adds the calling device's source address to ctx.
//
// This service terminates the device's connection, so the Auth Service only
// ever sees this Proxy as its peer and cannot observe the address itself.
// Metadata the device sent does not carry over to the outgoing call, so a
// device cannot choose the address the Auth Service enforces.
func forwardClientSrcAddr(ctx context.Context) (context.Context, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, trace.Errorf("no peer address for the calling device")
	}
	return devicetrust.WithForwardedClientSrcAddr(ctx, p.Addr), nil
}
