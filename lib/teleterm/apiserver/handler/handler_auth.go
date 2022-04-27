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

	"github.com/gravitational/trace"
)

// Login logs in a user to a cluster
func (s *Handler) Login(ctx context.Context, req *api.LoginRequest) (*api.EmptyResponse, error) {
	cluster, err := s.DaemonService.ResolveCluster(req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Params == nil {
		return nil, trace.BadParameter("missing login parameters")
	}

	switch params := req.Params.(type) {
	case *api.LoginRequest_Local:
		if err := cluster.LocalLogin(ctx, params.Local.User, params.Local.Password, params.Local.Token); err != nil {
			return nil, trace.Wrap(err)
		}

		return &api.EmptyResponse{}, nil
	case *api.LoginRequest_Sso:
		if err := cluster.SSOLogin(ctx, params.Sso.ProviderType, params.Sso.ProviderName); err != nil {
			return nil, trace.Wrap(err)
		}

		return &api.EmptyResponse{}, nil
	default:
		return nil, trace.BadParameter("unsupported login parameters")
	}

}

// Logout logs a user out from a cluster
func (s *Handler) Logout(ctx context.Context, req *api.LogoutRequest) (*api.EmptyResponse, error) {
	if err := s.DaemonService.ClusterLogout(ctx, req.ClusterUri); err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.EmptyResponse{}, nil
}

// GetAuthSettings returns cluster auth preferences
func (s *Handler) GetAuthSettings(ctx context.Context, req *api.GetAuthSettingsRequest) (*api.AuthSettings, error) {
	cluster, err := s.DaemonService.ResolveCluster(req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	preferences, err := cluster.SyncAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result := &api.AuthSettings{
		PreferredMfa:     string(preferences.PreferredLocalMFA),
		SecondFactor:     string(preferences.SecondFactor),
		LocalAuthEnabled: preferences.LocalAuthEnabled,
		AuthProviders:    []*api.AuthProvider{},
	}

	for _, provider := range preferences.Providers {
		result.AuthProviders = append(result.AuthProviders, &api.AuthProvider{
			Type:        provider.Type,
			Name:        provider.Name,
			DisplayName: provider.DisplayName,
		})
	}

	return result, nil
}
