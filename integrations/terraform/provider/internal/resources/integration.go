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

	"github.com/gravitational/teleport/api/scopes"
	apitypes "github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// NewIntegrationDataSourceType returns the integration data source type.
func NewIntegrationDataSourceType() tfdriver.DataSourceType[apitypes.IntegrationV1, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.DataSourceType[apitypes.IntegrationV1, tfdriver.ScopeQualifiedNameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[apitypes.IntegrationV1, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewIntegrationClient(clientFromProvider(p))
		},
		Kind: apitypes.KindIntegration,
		Codec: tfdriver.DataSourceCodecFuncs[apitypes.IntegrationV1]{
			SchemaFunc:  tfschema.GenSchemaIntegrationV1,
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
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[apitypes.IntegrationV1, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewIntegrationClient(clientFromProvider(p))
		},
		Kind: apitypes.KindIntegration,
		Codec: tfdriver.ResourceCodecFuncs[apitypes.IntegrationV1]{
			SchemaFunc:   tfschema.GenSchemaIntegrationV1,
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
