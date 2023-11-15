// Copyright 2023 Gravitational, Inc
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

	teletermv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/teleterm/daemon"
)

func (s *Handler) GetFileServerConfig(ctx context.Context, req *teletermv1.GetFileServerConfigRequest) (*teletermv1.GetFileServerConfigResponse, error) {
	shares, err := s.DaemonService.GetFileServerConfig(ctx, req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pbShares := make([]*teletermv1.FileServerShare, len(shares))
	for k, v := range shares {
		pbShares = append(pbShares, &teletermv1.FileServerShare{
			Name:         k,
			Path:         v.Path,
			AllowAnyone:  v.AllowAnyone,
			AllowedUsers: v.AllowedUsersList,
			AllowedRoles: v.AllowedRolesList,
		})
	}

	return &teletermv1.GetFileServerConfigResponse{
		Config: &teletermv1.FileServerConfig{
			Shares: pbShares,
		},
	}, nil
}

func (s *Handler) SetFileServerConfig(ctx context.Context, req *teletermv1.SetFileServerConfigRequest) (*teletermv1.SetFileServerConfigResponse, error) {
	shares := make(map[string]daemon.FileServerShare, len(req.GetConfig().Shares))
	for _, s := range req.GetConfig().Shares {
		shares[s.Name] = daemon.FileServerShare{
			Path:             s.Path,
			AllowAnyone:      s.AllowAnyone,
			AllowedUsersList: s.AllowedUsers,
			AllowedRolesList: s.AllowedRoles,
		}
	}

	err := s.DaemonService.SetFileServerConfig(ctx, req.ClusterUri, shares)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &teletermv1.SetFileServerConfigResponse{}, nil
}
