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

package resources

import (
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"

	"github.com/gravitational/teleport/api/scopes"
	apitypes "github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// NewKubernetesClusterDataSourceType returns the app data source type.
func NewKubernetesClusterDataSourceType() tfdriver.DataSourceType[apitypes.KubernetesClusterV3, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.DataSourceType[apitypes.KubernetesClusterV3, tfdriver.ScopeQualifiedNameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[apitypes.KubernetesClusterV3, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewKubernetesClient(clientFromProvider(p))
		},
		Kind: apitypes.KindKubernetesCluster,
		Name: apitypes.KindKubernetesCluster,
		Codec: tfdriver.DataSourceCodecFuncs[apitypes.KubernetesClusterV3]{
			SchemaFunc:  tfschema.GenSchemaKubernetesClusterV3DataSource,
			ToStateFunc: tfschema.CopyKubernetesClusterV3ToTerraform,
		},
		Identifier: tfdriver.PossiblyUnscopedScopeQualifiedNameIdentifierFromPath(
			tfdriver.ScopeQualifiedPath{
				Name:  path.Root("metadata").AtName("name"),
				Scope: path.Root("scope"),
			},
		),
	}
}

// NewKubernetesClusterResourceType returns the app resource type.
func NewKubernetesClusterResourceType() tfdriver.ResourceType[apitypes.KubernetesClusterV3, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.ResourceType[apitypes.KubernetesClusterV3, tfdriver.ScopeQualifiedNameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[apitypes.KubernetesClusterV3, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewKubernetesClient(clientFromProvider(p))
		},
		Kind: apitypes.KindKubernetesCluster,
		Name: apitypes.KindKubernetesCluster,
		Codec: tfdriver.ResourceCodecFuncs[apitypes.KubernetesClusterV3]{
			SchemaFunc:   tfschema.GenSchemaKubernetesClusterV3Resource,
			ToStateFunc:  tfschema.CopyKubernetesClusterV3ToTerraform,
			FromPlanFunc: tfschema.CopyKubernetesClusterV3FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[apitypes.KubernetesClusterV3](),
		Identifier: tfdriver.PossiblyUnscopedScopeQualifiedNameIdentifierPolicy(
			tfdriver.ScopeQualifiedPath{
				Name:  path.Root("metadata").AtName("name"),
				Scope: path.Root("scope"),
			},
			func(av *apitypes.KubernetesClusterV3) scopes.QualifiedName {
				return scopes.QualifiedName{
					Name:  av.GetMetadata().Name,
					Scope: av.GetScope(),
				}
			}),
		ResourceRevision: func(st *apitypes.KubernetesClusterV3) string {
			return st.GetMetadata().Revision
		},
	}
}
