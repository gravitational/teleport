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

package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/gravitational/trace"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/teleterm/cmd"
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

	apiGateway, err := s.newAPIGateway(ctx, gateway)
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
		apiGateway, err := s.newAPIGateway(ctx, gw)
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

func (s *Handler) newAPIGateway(ctx context.Context, gateway gateway.Gateway) (*api.Gateway, error) {
	cmds, err := s.DaemonService.GetGatewayCLICommand(ctx, gateway)
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
		GatewayCliCommand:     makeGatewayCLICommand(cmds),
	}, nil
}

func makeGatewayCLICommand(cmds cmd.Cmds) *api.GatewayCLICommand {
	preview := strings.TrimSpace(
		fmt.Sprintf("%s %s",
			strings.Join(cmds.Preview.Env, " "),
			strings.Join(cmds.Preview.Args, " ")))

	return &api.GatewayCLICommand{
		Path:    cmds.Exec.Path,
		Args:    cmds.Exec.Args,
		Env:     cmds.Exec.Env,
		Preview: preview,
	}
}

// SetGatewayTargetSubresourceName changes the TargetSubresourceName field of gateway.Gateway
// and returns the updated version of gateway.Gateway.
//
// In Connect this is used to update the db name of a db connection along with the CLI command.
func (s *Handler) SetGatewayTargetSubresourceName(ctx context.Context, req *api.SetGatewayTargetSubresourceNameRequest) (*api.Gateway, error) {
	gateway, err := s.DaemonService.SetGatewayTargetSubresourceName(ctx, req.GatewayUri, req.TargetSubresourceName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	apiGateway, err := s.newAPIGateway(ctx, gateway)
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

	apiGateway, err := s.newAPIGateway(ctx, gateway)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return apiGateway, nil
}
