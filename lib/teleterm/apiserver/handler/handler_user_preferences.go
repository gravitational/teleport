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

func (s *Handler) UpdateUserPreferences(ctx context.Context, req *api.UpdateUserPreferencesRequest) (*api.EmptyResponse, error) {
	clusterURI, err := uri.Parse(req.GetClusterUri())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.DaemonService.UpdateUserPreferences(ctx, clusterURI, req.GetUserPreferences())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.EmptyResponse{}, nil
}
