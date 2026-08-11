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

package resources

import (
	"github.com/gravitational/teleport/api/types"
	"github.com/hashicorp/terraform-plugin-framework/provider"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// clusterNetworkingConfigID is the static identifier used in the terraform state.
// Note that types.MetaNameClusterNetworkingConfig is "cluster-networking-config"
// which can't be used here without causing churn.
const clusterNetworkingConfigID = "cluster_networking_config"

// NewClusterNetworkingConfigDataSourceType returns the cluster networking config data source type.
func NewClusterNetworkingConfigDataSourceType() tfdriver.DataSourceType[types.ClusterNetworkingConfigV2, tfdriver.SingletonIdentifier] {
	return tfdriver.DataSourceType[types.ClusterNetworkingConfigV2, tfdriver.SingletonIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[types.ClusterNetworkingConfigV2, tfdriver.SingletonIdentifier] {
			return teleport.NewClusterNetworkingConfigClient(clientFromProvider(p))
		},
		Kind: types.KindClusterNetworkingConfig,
		Name: types.KindClusterNetworkingConfig,
		Codec: tfdriver.DataSourceCodecFuncs[types.ClusterNetworkingConfigV2]{
			SchemaFunc:  tfschema.GenSchemaClusterNetworkingConfigV2,
			ToStateFunc: tfschema.CopyClusterNetworkingConfigV2ToTerraform,
		},
		Identifier: tfdriver.SingletonIdentifierFromName(clusterNetworkingConfigID),
	}
}

// NewClusterNetworkingConfigResourceType returns the cluster networking config resource type.
func NewClusterNetworkingConfigResourceType() tfdriver.ResourceType[types.ClusterNetworkingConfigV2, tfdriver.SingletonIdentifier] {
	return tfdriver.ResourceType[types.ClusterNetworkingConfigV2, tfdriver.SingletonIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[types.ClusterNetworkingConfigV2, tfdriver.SingletonIdentifier] {
			return teleport.NewClusterNetworkingConfigClient(clientFromProvider(p))
		},
		Kind: types.KindClusterNetworkingConfig,
		Name: types.KindClusterNetworkingConfig,
		Codec: tfdriver.ResourceCodecFuncs[types.ClusterNetworkingConfigV2]{
			SchemaFunc:   tfschema.GenSchemaClusterNetworkingConfigV2,
			ToStateFunc:  tfschema.CopyClusterNetworkingConfigV2ToTerraform,
			FromPlanFunc: tfschema.CopyClusterNetworkingConfigV2FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[types.ClusterNetworkingConfigV2](),
		Identifier: tfdriver.SingletonIdentifierPolicy[types.ClusterNetworkingConfigV2](clusterNetworkingConfigID),
		ResourceRevision: func(st *types.ClusterNetworkingConfigV2) string {
			return st.GetMetadata().Revision
		},
	}
}
