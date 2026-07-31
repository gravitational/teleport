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
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"

	apitypes "github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// NewKubernetesClusterDataSourceType returns the app data source type.
func NewKubernetesClusterDataSourceType() tfdriver.DataSourceType[apitypes.KubernetesClusterV3, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.DataSourceType[apitypes.KubernetesClusterV3, tfdriver.ScopeQualifiedNameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[apitypes.KubernetesClusterV3, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewKubernetesClient(clientFromProvider(p))
		},
		Kind: apitypes.KindKubernetesCluster,
		Codec: tfdriver.DataSourceCodecFuncs[apitypes.KubernetesClusterV3]{
			SchemaFunc:  tfschema.GenSchemaKubernetesClusterV3,
			ToStateFunc: tfschema.CopyKubernetesClusterV3ToTerraform,
		},
		Identifier: tfdriver.PossiblyUnscopedScopeQualifiedNameIdentifierFromPath(
			path.Root("metadata").AtName("name"),
			path.Root("scope"),
		),
	}
}

// NewKubernetesClusterResourceType returns the app resource type.
func NewKubernetesClusterResourceType() tfdriver.ResourceType[apitypes.KubernetesClusterV3, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.ResourceType[apitypes.KubernetesClusterV3, tfdriver.ScopeQualifiedNameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[apitypes.KubernetesClusterV3, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewKubernetesClient(clientFromProvider(p))
		},
		Kind: apitypes.KindKubernetesCluster,
		Codec: tfdriver.ResourceCodecFuncs[apitypes.KubernetesClusterV3]{
			SchemaFunc:   tfschema.GenSchemaKubernetesClusterV3,
			ToStateFunc:  tfschema.CopyKubernetesClusterV3ToTerraform,
			FromPlanFunc: tfschema.CopyKubernetesClusterV3FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[apitypes.KubernetesClusterV3](),
		Identifier: tfdriver.PossiblyUnscopedScopeQualifiedNameIdentifierPolicy(
			path.Root("metadata").AtName("name"),
			path.Root("scope"),
			func(av *apitypes.KubernetesClusterV3) (string, string) {
				return av.GetMetadata().Name, av.GetScope()
			}),
		ResourceRevision: func(st *apitypes.KubernetesClusterV3) string {
			return st.GetMetadata().Revision
		},
	}
}
