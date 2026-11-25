// Copyright 2026 Gravitational, Inc.
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

package legacy

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// DataSourceType is a data source type used to instantiate a new DataSource.
type DataSourceType interface {
	NewDataSource(context.Context, provider.Provider) (datasource.DataSource, diag.Diagnostics)
}

// ResourceType is a resource type used to instantiate a new Resource.
type ResourceType interface {
	NewResource(context.Context, provider.Provider) (resource.Resource, diag.Diagnostics)
}

// DataSources returns all legacy Teleport data sources.
func DataSources(ctx context.Context, p provider.Provider) []func() datasource.DataSource {
	return DataSourcesFromTypes(ctx, p, dataSourceTypes())
}

// dataSourceTypes returns all legacy Teleport data source types.
func dataSourceTypes() []DataSourceType {
	return []DataSourceType{
		dataSourceTeleportClusterMaintenanceConfigType{},
		dataSourceTeleportDiscoveryConfigType{},
		dataSourceTeleportGithubConnectorType{},
		dataSourceTeleportProvisionTokenType{},
		dataSourceTeleportOIDCConnectorType{},
		dataSourceTeleportSAMLIdPServiceProviderType{},
		dataSourceTeleportAutoUpdateConfigType{},
		dataSourceTeleportVnetConfigType{},
		// TODO(bl-nero): Add teleport_inference_* data sources after data sources
		// are fixed. The current problems with data sources include:
		// - Data sources only perform a "shallow fill", which means only setting
		//   leaf-level fields.
		// - Data sources use the same schema as resources, which means that fields
		//   required on a resource also need to be set on the data source
		//   definition.
		dataSourceTeleportWorkloadClusterType{},
		dataSourceTeleportClientIPRestrictionType{},
	}
}

// DataSourcesFromTypes returns data sources from types
func DataSourcesFromTypes(ctx context.Context, p provider.Provider, dtypes []DataSourceType) []func() datasource.DataSource {
	dataSources := make([]func() datasource.DataSource, 0, len(dtypes))
	for _, dtype := range dtypes {
		dataSources = append(dataSources, func() datasource.DataSource {
			// Returned diagnostics is always nil
			dataSource, _ := dtype.NewDataSource(ctx, p)
			return dataSource
		})
	}
	return dataSources
}

// Resources returns all legacy Teleport resources.
func Resources(ctx context.Context, p provider.Provider) []func() resource.Resource {
	return ResourcesFromTypes(ctx, p, resourceTypes())
}

// resourceTypes returns all legacy Teleport resource types.
func resourceTypes() []ResourceType {
	return []ResourceType{
		resourceTeleportClusterMaintenanceConfigType{},
		resourceTeleportDiscoveryConfigType{},
		resourceTeleportGithubConnectorType{},
		resourceTeleportProvisionTokenType{},
		resourceTeleportOIDCConnectorType{},
		resourceTeleportSAMLIdPServiceProviderType{},
		resourceTeleportBotType{},
		resourceTeleportAutoUpdateConfigType{},
		resourceTeleportVnetConfigType{},
		resourceTeleportInferenceModelType{},
		resourceTeleportInferenceSecretType{},
		resourceTeleportInferencePolicyType{},
		resourceTeleportRetrievalModelType{},
		resourceTeleportWorkloadClusterType{},
		resourceTeleportClientIPRestrictionType{},
	}
}

// ResourcesFromTypes returns resources from types
func ResourcesFromTypes(ctx context.Context, p provider.Provider, rtypes []ResourceType) []func() resource.Resource {
	resources := make([]func() resource.Resource, 0, len(rtypes))
	for _, rtype := range rtypes {
		resources = append(resources, func() resource.Resource {
			// Returned diagnostics is always nil
			resource, _ := rtype.NewResource(ctx, p)
			return resource
		})
	}
	return resources
}
