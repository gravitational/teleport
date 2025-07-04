// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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
	"github.com/gravitational/teleport/lib/autoupdate/tools"
)

// GetAutoUpdateVersions returns auto update version for clusters that are reachable.
func (s *Handler) GetAutoUpdateVersions(ctx context.Context, _ *api.GetAutoUpdateVersionsRequest) (*api.GetAutoUpdateVersionsResponse, error) {
	versions, err := s.DaemonService.GetAutoUpdateVersions(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.GetAutoUpdateVersionsResponse{
		Versions: versions,
	}, nil
}

// GetAutoUpdateBaseUrl returns base URL for downloading Teleport packages.
func (s *Handler) GetAutoUpdateBaseUrl(_ context.Context, _ *api.GetAutoUpdateBaseUrlRequest) (*api.GetAutoUpdateBaseUrlResponse, error) {
	baseUrl, err := tools.ResolveBaseURL()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.GetAutoUpdateBaseUrlResponse{
		BaseUrl: baseUrl,
	}, nil
}
