// Teleport
// Copyright (C) 2023  Gravitational, Inc.
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
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

func (s *Handler) GetUserPreferences(ctx context.Context, req *api.GetUserPreferencesRequest) (*api.GetUserPreferencesResponse, error) {
	clusterURI, err := uri.Parse(req.GetClusterUri())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	preferences, err := s.DaemonService.GetUserPreferences(ctx, clusterURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.GetUserPreferencesResponse{UserPreferences: preferences}, nil
}

func (s *Handler) UpdateUserPreferences(ctx context.Context, req *api.UpdateUserPreferencesRequest) (*api.UpdateUserPreferencesResponse, error) {
	clusterURI, err := uri.Parse(req.GetClusterUri())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updated, err := s.DaemonService.UpdateUserPreferences(ctx, clusterURI, req.GetUserPreferences())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.UpdateUserPreferencesResponse{
		UserPreferences: updated,
	}, nil
}
