// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package legacy

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// DataSources returns all legacy Teleport data sources.
func DataSources(p provider.Provider) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		func() datasource.DataSource { return NewDataSourceClusterMaintenanceConfig(p) },
		func() datasource.DataSource { return NewDataSourceDiscoveryConfig(p) },
		func() datasource.DataSource { return NewDataSourceGithubConnector(p) },
		func() datasource.DataSource { return NewDataSourceProvisionToken(p) },
		func() datasource.DataSource { return NewDataSourceOIDCConnector(p) },
		func() datasource.DataSource { return NewDataSourceSAMLIdPServiceProvider(p) },
		func() datasource.DataSource { return NewDataSourceAutoUpdateConfig(p) },
		func() datasource.DataSource { return NewDataSourceVnetConfig(p) },
		func() datasource.DataSource { return NewDataSourceIntegration(p) },
		// TODO(bl-nero): Add teleport_inference_* data sources after data sources
		// are fixed. The current problems with data sources include:
		// - Data sources only perform a "shallow fill", which means only setting
		//   leaf-level fields.
		// - Data sources use the same schema as resources, which means that fields
		//   required on a resource also need to be set on the data source
		//   definition.
		func() datasource.DataSource { return NewDataSourceWorkloadCluster(p) },
		func() datasource.DataSource { return NewDataSourceClientIPRestriction(p) },
	}
}

// Resources returns all legacy Teleport resource types.
func Resources(p provider.Provider) []func() resource.Resource {
	return []func() resource.Resource{
		func() resource.Resource { return NewResourceClusterMaintenanceConfig(p) },
		func() resource.Resource { return NewResourceDiscoveryConfig(p) },
		func() resource.Resource { return NewResourceGithubConnector(p) },
		func() resource.Resource { return NewResourceProvisionToken(p) },
		func() resource.Resource { return NewResourceOIDCConnector(p) },
		func() resource.Resource { return NewResourceSAMLIdPServiceProvider(p) },
		func() resource.Resource { return NewResourceBot(p) },
		func() resource.Resource { return NewResourceAutoUpdateConfig(p) },
		func() resource.Resource { return NewResourceVnetConfig(p) },
		func() resource.Resource { return NewResourceIntegration(p) },
		func() resource.Resource { return NewResourceInferenceModel(p) },
		func() resource.Resource { return NewResourceInferenceSecret(p) },
		func() resource.Resource { return NewResourceInferencePolicy(p) },
		func() resource.Resource { return NewResourceRetrievalModel(p) },
		func() resource.Resource { return NewResourceWorkloadCluster(p) },
		func() resource.Resource { return NewResourceClientIPRestriction(p) },
	}
}
