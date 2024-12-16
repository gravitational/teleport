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
	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/ui"
)

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

	apiLabels := makeAPILabels(ui.MakeLabelsWithoutInternalPrefixes(app.GetAllLabels()))

	tcpPorts := make([]*api.PortRange, 0, len(app.GetTCPPorts()))
	for _, portRange := range app.GetTCPPorts() {
		tcpPorts = append(tcpPorts, &api.PortRange{Port: portRange.Port, EndPort: portRange.EndPort})
	}

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
		TcpPorts:     tcpPorts,
	}
}

func newSAMLIdPServiceProviderAPIApp(clusterApp clusters.SAMLIdPServiceProvider) *api.App {
	provider := clusterApp.Provider
	apiLabels := makeAPILabels(ui.MakeLabelsWithoutInternalPrefixes(provider.GetAllLabels()))

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
