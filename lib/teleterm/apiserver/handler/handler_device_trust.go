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
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

func (s *Handler) AuthenticateWebDevice(ctx context.Context, req *api.AuthenticateWebDeviceRequest) (*api.AuthenticateWebDeviceResponse, error) {
	switch {
	case req.RootClusterUri == "":
		return nil, trace.BadParameter("root_cluster_uri required")
	case req.DeviceWebToken.GetId() == "":
		return nil, trace.BadParameter("device_web_token.id required")
	case req.DeviceWebToken.GetToken() == "":
		return nil, trace.BadParameter("device_web_token.token required")
	}

	clusterURI, err := uri.Parse(req.GetRootClusterUri())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := s.DaemonService.AuthenticateWebDevice(ctx, clusterURI, req)
	return resp, trace.Wrap(err)
}
