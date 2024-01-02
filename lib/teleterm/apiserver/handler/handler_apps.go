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
	"sort"

	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
)

func newAPIApp(clusterApp clusters.App) *api.App {
	app := clusterApp.App
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
