// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package handler

import (
	"context"

	"github.com/gravitational/trace"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/vnet"
)

func (s *Handler) StartVnet(ctx context.Context, req *api.StartVnetRequest) (*api.StartVnetResponse, error) {
	// TODO: Take req.RootClusterUri into account.
	if s.Vnet != nil {
		return nil, trace.CompareFailed("vnet is already running")
	}

	tun, err := vnet.CreateAndSetupTUNDevice(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, client, err := s.DaemonService.ResolveCluster(req.RootClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	vnetCtx, vnetCancel := context.WithCancel(s.DaemonService.RootContext)
	manager, err := vnet.NewManager(vnetCtx, &vnet.Config{
		Client:    client,
		TUNDevice: tun,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.Vnet = manager
	s.VnetCancel = vnetCancel

	go func() {
		s.Vnet.Run()
		// TODO: Log error.
	}()

	return &api.StartVnetResponse{}, nil
}

func (s *Handler) StopVnet(ctx context.Context, req *api.StopVnetRequest) (*api.StopVnetResponse, error) {
	if s.Vnet == nil {
		return nil, trace.Errorf("vnet is not running")
	}

	s.VnetCancel()
	// TODO: We probably don't want to nil this.
	s.Vnet = nil
	s.VnetCancel = nil

	return &api.StopVnetResponse{}, nil
}
