// Copyright 2021 Gravitational, Inc
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

package handler

import (
	"context"

	"github.com/gravitational/trace"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/teleterm/daemon"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
)

// CreateGateway creates a gateway
func (s *Handler) CreateGateway(ctx context.Context, req *api.CreateGatewayRequest) (*api.Gateway, error) {
	params := daemon.CreateGatewayParams{
		TargetURI:             req.TargetUri,
		TargetUser:            req.TargetUser,
		TargetSubresourceName: req.TargetSubresourceName,
		LocalPort:             req.LocalPort,
	}

	gateway, err := s.DaemonService.CreateGateway(ctx, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	apiGateway, err := newAPIGateway(gateway)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return apiGateway, nil
}

// ListGateways lists all gateways
func (s *Handler) ListGateways(ctx context.Context, req *api.ListGatewaysRequest) (*api.ListGatewaysResponse, error) {
	gws := s.DaemonService.ListGateways()

	apiGws := make([]*api.Gateway, 0, len(gws))
	for _, gw := range gws {
		apiGateway, err := newAPIGateway(gw)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		apiGws = append(apiGws, apiGateway)
	}

	return &api.ListGatewaysResponse{
		Gateways: apiGws,
	}, nil
}

// RemoveGateway removes cluster gateway
func (s *Handler) RemoveGateway(ctx context.Context, req *api.RemoveGatewayRequest) (*api.EmptyResponse, error) {
	if err := s.DaemonService.RemoveGateway(req.GatewayUri); err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.EmptyResponse{}, nil
}

func newAPIGateway(gateway gateway.Gateway) (*api.Gateway, error) {
	command, err := gateway.CLICommand()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.Gateway{
		Uri:                   gateway.URI().String(),
		TargetUri:             gateway.TargetURI().String(),
		TargetName:            gateway.TargetName(),
		TargetUser:            gateway.TargetUser(),
		TargetSubresourceName: gateway.TargetSubresourceName(),
		Protocol:              gateway.Protocol(),
		LocalAddress:          gateway.LocalAddress(),
		LocalPort:             gateway.LocalPort(),
		GatewayCliCommand:     command,
	}, nil
}

// SetGatewayTargetSubresourceName changes the TargetSubresourceName field of gateway.Gateway
// and returns the updated version of gateway.Gateway.
//
// In Connect this is used to update the db name of a db connection along with the CLI command.
func (s *Handler) SetGatewayTargetSubresourceName(ctx context.Context, req *api.SetGatewayTargetSubresourceNameRequest) (*api.Gateway, error) {
	gateway, err := s.DaemonService.SetGatewayTargetSubresourceName(req.GatewayUri, req.TargetSubresourceName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	apiGateway, err := newAPIGateway(gateway)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return apiGateway, nil
}

// SetGatewayLocalPort restarts the gateway under the new port without fetching new certs.
func (s *Handler) SetGatewayLocalPort(ctx context.Context, req *api.SetGatewayLocalPortRequest) (*api.Gateway, error) {
	gateway, err := s.DaemonService.SetGatewayLocalPort(req.GatewayUri, req.LocalPort)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	apiGateway, err := newAPIGateway(gateway)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return apiGateway, nil
}
