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

// NewIntegrationDataSourceType returns the integration data source type.
func NewIntegrationDataSourceType() tfdriver.DataSourceType[apitypes.IntegrationV1, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.DataSourceType[apitypes.IntegrationV1, tfdriver.ScopeQualifiedNameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[apitypes.IntegrationV1, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewIntegrationClient(clientFromProvider(p))
		},
		Kind: apitypes.KindIntegration,
		Name: apitypes.KindIntegration,
		Codec: tfdriver.DataSourceCodecFuncs[apitypes.IntegrationV1]{
			SchemaFunc:  tfschema.GenSchemaIntegrationV1DataSource,
			ToStateFunc: tfschema.CopyIntegrationV1ToTerraform,
		},
		Identifier: tfdriver.PossiblyUnscopedScopeQualifiedNameIdentifierFromPath(
			tfdriver.ScopeQualifiedPath{
				Name:  path.Root("metadata").AtName("name"),
				Scope: path.Root("scope"),
			},
		),
	}
}

// NewIntegrationResourceType returns the integration resource type.
func NewIntegrationResourceType() tfdriver.ResourceType[apitypes.IntegrationV1, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.ResourceType[apitypes.IntegrationV1, tfdriver.ScopeQualifiedNameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[apitypes.IntegrationV1, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewIntegrationClient(clientFromProvider(p))
		},
		Kind: apitypes.KindIntegration,
		Name: apitypes.KindIntegration,
		Codec: tfdriver.ResourceCodecFuncs[apitypes.IntegrationV1]{
			SchemaFunc:   tfschema.GenSchemaIntegrationV1Resource,
			ToStateFunc:  tfschema.CopyIntegrationV1ToTerraform,
			FromPlanFunc: tfschema.CopyIntegrationV1FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[apitypes.IntegrationV1](),
		Identifier: tfdriver.PossiblyUnscopedScopeQualifiedNameIdentifierPolicy(
			tfdriver.ScopeQualifiedPath{
				Name:  path.Root("metadata").AtName("name"),
				Scope: path.Root("scope"),
			},
			func(ig *apitypes.IntegrationV1) scopes.QualifiedName {
				return scopes.QualifiedName{
					Name:  ig.GetMetadata().Name,
					Scope: ig.GetScope(),
				}
			}),
		ResourceRevision: func(st *apitypes.IntegrationV1) string {
			return st.GetMetadata().Revision
		},
	}
}
