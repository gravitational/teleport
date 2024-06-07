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
	"time"

	"github.com/gravitational/trace"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

func (s *Handler) CreateConnectMyComputerRole(ctx context.Context, req *api.CreateConnectMyComputerRoleRequest) (*api.CreateConnectMyComputerRoleResponse, error) {
	res, err := s.DaemonService.CreateConnectMyComputerRole(ctx, req)
	return res, trace.Wrap(err)
}

func (s *Handler) CreateConnectMyComputerNodeToken(ctx context.Context, req *api.CreateConnectMyComputerNodeTokenRequest) (*api.CreateConnectMyComputerNodeTokenResponse, error) {
	token, err := s.DaemonService.CreateConnectMyComputerNodeToken(ctx, req.GetRootClusterUri())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := &api.CreateConnectMyComputerNodeTokenResponse{
		Token: token,
	}

	return response, nil
}

func (s *Handler) WaitForConnectMyComputerNodeJoin(ctx context.Context, req *api.WaitForConnectMyComputerNodeJoinRequest) (*api.WaitForConnectMyComputerNodeJoinResponse, error) {
	// The Electron app aborts the request after a timeout that's much shorter. However, we're going
	// to add an internal timeout as well to protect from requests hanging forever if a client doesn't
	// set a deadline or doesn't abort the request.
	timeoutCtx, close := context.WithTimeout(ctx, time.Minute)
	defer close()

	rootClusterURI, err := uri.Parse(req.RootClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	server, err := s.DaemonService.WaitForConnectMyComputerNodeJoin(timeoutCtx, rootClusterURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.WaitForConnectMyComputerNodeJoinResponse{
		Server: newAPIServer(server),
	}, err
}

func (s *Handler) DeleteConnectMyComputerNode(ctx context.Context, req *api.DeleteConnectMyComputerNodeRequest) (*api.DeleteConnectMyComputerNodeResponse, error) {
	res, err := s.DaemonService.DeleteConnectMyComputerNode(ctx, req)
	return res, trace.Wrap(err)
}

func (s *Handler) GetConnectMyComputerNodeName(ctx context.Context, req *api.GetConnectMyComputerNodeNameRequest) (*api.GetConnectMyComputerNodeNameResponse, error) {
	res, err := s.DaemonService.GetConnectMyComputerNodeName(req)
	return res, trace.Wrap(err)
}
