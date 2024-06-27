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

	"github.com/gravitational/trace"

	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
)

// Login logs in a user to a cluster
func (s *Handler) Login(ctx context.Context, req *api.LoginRequest) (*api.EmptyResponse, error) {
	cluster, clusterClient, err := s.DaemonService.ResolveCluster(req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cluster.URI.IsRoot() {
		return nil, trace.BadParameter("cluster URI must be a root URI")
	}

	// The credentials + MFA login flow in the Electron app assumes that the default CLI prompt is
	// used and works around that. Thus we have to remove the teleterm-specific MFAPromptConstructor
	// added by daemon.Service.ResolveClusterURI.
	clusterClient.MFAPromptConstructor = nil

	if err = s.DaemonService.ClearCachedClientsForRoot(cluster.URI); err != nil {
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
	case *api.LoginRequest_Sso:
		if err := cluster.SSOLogin(ctx, params.Sso.ProviderType, params.Sso.ProviderName); err != nil {
			return nil, trace.Wrap(err)
		}
	default:
		return nil, trace.BadParameter("unsupported login parameters")
	}

	// Don't wait for the headless watcher to initialize as this could slow down logins.
	if err := s.DaemonService.StartHeadlessWatcher(req.ClusterUri, false /* waitInit */); err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.EmptyResponse{}, nil
}

// LoginPasswordless logs in a user to a cluster passwordlessly.
func (s *Handler) LoginPasswordless(stream api.TerminalService_LoginPasswordlessServer) error {
	// Init stream request with cluster uri.
	req, err := stream.Recv()
	if err != nil {
		return trace.Wrap(err)
	}

	initReq := req.GetInit()
	if initReq == nil || initReq.GetClusterUri() == "" {
		return trace.BadParameter("cluster URI is required")
	}

	cluster, clusterClient, err := s.DaemonService.ResolveCluster(initReq.GetClusterUri())
	if err != nil {
		return trace.Wrap(err)
	}

	if !cluster.URI.IsRoot() {
		return trace.BadParameter("cluster URI must be a root URI")
	}

	// The passwordless login flow in the Electron app assumes that the default CLI prompt is used and
	// works around that. Thus we have to remove the teleterm-specific MFAPromptConstructor added by
	// daemon.Service.ResolveClusterURI.
	clusterClient.MFAPromptConstructor = nil

	if err := s.DaemonService.ClearCachedClientsForRoot(cluster.URI); err != nil {
		return trace.Wrap(err)
	}

	// Start the prompt flow.
	if err := cluster.PasswordlessLogin(stream.Context(), stream); err != nil {
		return trace.Wrap(err)
	}

	// Don't wait for the headless watcher to initialize as this could slow down logins.
	if err := s.DaemonService.StartHeadlessWatcher(initReq.GetClusterUri(), false /* waitInit */); err != nil {
		return trace.Wrap(err)
	}

	return nil
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
	cluster, _, err := s.DaemonService.ResolveCluster(req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	preferences, err := cluster.SyncAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result := &api.AuthSettings{
		PreferredMfa:       string(preferences.PreferredLocalMFA),
		SecondFactor:       string(preferences.SecondFactor),
		LocalAuthEnabled:   preferences.LocalAuthEnabled,
		AuthType:           preferences.AuthType,
		AllowPasswordless:  preferences.AllowPasswordless,
		LocalConnectorName: preferences.LocalConnectorName,
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
