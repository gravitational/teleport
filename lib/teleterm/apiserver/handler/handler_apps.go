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
	"sort"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
)

// GetApps gets apps with filters and returns paginated results
func (s *Handler) GetApps(ctx context.Context, req *api.GetAppsRequest) (*api.GetAppsResponse, error) {
	cluster, _, err := s.DaemonService.ResolveCluster(req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyClient, err := s.DaemonService.GetCachedClient(ctx, cluster.URI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := cluster.GetApps(ctx, proxyClient.CurrentCluster(), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := &api.GetAppsResponse{
		StartKey:   resp.StartKey,
		TotalCount: int32(resp.TotalCount),
	}

	for _, app := range resp.Apps {
		var apiApp *api.App
		if app.App != nil {
			apiApp = newAPIApp(*app.App)
		} else if app.SAMLIdPServiceProvider != nil {
			apiApp = newSAMLIdPServiceProviderAPIApp(*app.SAMLIdPServiceProvider)
		} else {
			return nil, trace.Errorf("expected an app server or a SAML IdP provider")
		}
		response.Agents = append(response.Agents, apiApp)
	}

	return response, nil
}

func newAPIApp(clusterApp clusters.App) *api.App {
	app := clusterApp.App

	awsRoles := []*api.AWSRole{}
	for _, role := range clusterApp.AWSRoles {
		awsRoles = append(awsRoles, &api.AWSRole{
			Name:      role.Name,
			Display:   role.Display,
			Arn:       role.ARN,
			AccountId: role.AccountID,
		})
	}

	apiLabels := APILabels{}
	for name, value := range app.GetAllLabels() {
		apiLabels = append(apiLabels, &api.Label{
			Name:  name,
			Value: value,
		})
	}
	sort.Sort(apiLabels)

	return &api.App{
		Uri:          clusterApp.URI.String(),
		EndpointUri:  app.GetURI(),
		Name:         app.GetName(),
		Desc:         app.GetDescription(),
		AwsConsole:   app.IsAWSConsole(),
		PublicAddr:   app.GetPublicAddr(),
		Fqdn:         clusterApp.FQDN,
		AwsRoles:     awsRoles,
		FriendlyName: types.FriendlyName(app),
		SamlApp:      false,
		Labels:       apiLabels,
	}
}

func newSAMLIdPServiceProviderAPIApp(clusterApp clusters.SAMLIdPServiceProvider) *api.App {
	provider := clusterApp.Provider
	apiLabels := APILabels{}
	for name, value := range provider.GetAllLabels() {
		apiLabels = append(apiLabels, &api.Label{
			Name:  name,
			Value: value,
		})
	}
	sort.Sort(apiLabels)

	// Keep in sync with lib/web/ui/app.go.
	return &api.App{
		Uri:          clusterApp.URI.String(),
		Name:         provider.GetName(),
		Desc:         "SAML Application",
		PublicAddr:   "",
		FriendlyName: types.FriendlyName(provider),
		SamlApp:      true,
		Labels:       apiLabels,
	}
}
