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

	"github.com/gravitational/teleport"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/client"
)

// Login logs in a user to a cluster
func (s *Handler) Login(ctx context.Context, req *api.LoginRequest) (*api.EmptyResponse, error) {
	cluster, _, err := s.DaemonService.ResolveCluster(req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cluster.URI.IsRoot() {
		return nil, trace.BadParameter("cluster URI must be a root URI")
	}

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

	preferences, pr, err := cluster.SyncAuthPreference(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result := &api.AuthSettings{
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

	versions := client.Versions{
		MinClient: pr.MinClientVersion,
		Client:    teleport.Version,
		Server:    pr.ServerVersion,
	}

	clientVersionStatus, err := client.GetClientVersionStatus(versions)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result.ClientVersionStatus = libclientVersionStatusToAPIVersionStatus(clientVersionStatus)
	result.Versions = &api.Versions{
		MinClient: versions.MinClient,
		Client:    versions.Client,
		Server:    versions.Server,
	}

	return result, nil
}

// StartHeadlessWatcher starts a headless watcher.
// If the watcher is already running, it is restarted.
func (s *Handler) StartHeadlessWatcher(_ context.Context, req *api.StartHeadlessWatcherRequest) (*api.StartHeadlessWatcherResponse, error) {
	// Don't wait for the headless watcher to initialize
	err := s.DaemonService.StartHeadlessWatcher(req.RootClusterUri, false /* waitInit */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.StartHeadlessWatcherResponse{}, nil
}

func libclientVersionStatusToAPIVersionStatus(vs client.ClientVersionStatus) api.ClientVersionStatus {
	switch vs {
	case client.ClientVersionOK:
		return api.ClientVersionStatus_CLIENT_VERSION_STATUS_OK

	case client.ClientVersionTooOld:
		return api.ClientVersionStatus_CLIENT_VERSION_STATUS_TOO_OLD

	case client.ClientVersionTooNew:
		return api.ClientVersionStatus_CLIENT_VERSION_STATUS_TOO_NEW
	}

	return api.ClientVersionStatus_CLIENT_VERSION_STATUS_COMPAT_UNSPECIFIED
}
