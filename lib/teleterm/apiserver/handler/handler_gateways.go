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

	api "github.com/gravitational/teleport/lib/teleterm/api/protogen/golang/v1"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/gateway"

	"github.com/gravitational/trace"
)

// CreateGateway creates a gateway
func (s *Handler) CreateGateway(ctx context.Context, req *api.CreateGatewayRequest) (*api.Gateway, error) {
	params := clusters.CreateGatewayParams{
		TargetURI:  req.TargetUri,
		TargetUser: req.TargetUser,
		LocalPort:  req.LocalPort,
	}

	gateway, err := s.DaemonService.CreateGateway(ctx, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newAPIGateway(gateway), nil
}

// ListGateways lists all gateways
func (s *Handler) ListGateways(ctx context.Context, req *api.ListGatewaysRequest) (*api.ListGatewaysResponse, error) {
	gws, err := s.DaemonService.ListGateways(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	apiGws := []*api.Gateway{}
	for _, gw := range gws {
		apiGws = append(apiGws, newAPIGateway(gw))
	}

	return &api.ListGatewaysResponse{
		Gateways: apiGws,
	}, nil
}

// RemoveGateway removes cluster gateway
func (s *Handler) RemoveGateway(ctx context.Context, req *api.RemoveGatewayRequest) (*api.EmptyResponse, error) {
	if err := s.DaemonService.RemoveGateway(ctx, req.GatewayUri); err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.EmptyResponse{}, nil
}

func newAPIGateway(gateway *gateway.Gateway) *api.Gateway {
	return &api.Gateway{
		Uri:          gateway.URI.String(),
		TargetUri:    gateway.TargetURI,
		TargetName:   gateway.TargetName,
		TargetUser:   gateway.TargetUser,
		Protocol:     gateway.Protocol,
		LocalAddress: gateway.LocalAddress,
		LocalPort:    gateway.LocalPort,
		CaCertPath:   gateway.CACertPath,
		CertPath:     gateway.CertPath,
		KeyPath:      gateway.KeyPath,
		Insecure:     gateway.Insecure,
	}
}
