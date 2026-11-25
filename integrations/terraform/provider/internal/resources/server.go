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

// NewSSHServerDataSourceType returns the SSH server data source type.
func NewSSHServerDataSourceType() tfdriver.DataSourceType[apitypes.ServerV2, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.DataSourceType[apitypes.ServerV2, tfdriver.ScopeQualifiedNameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[apitypes.ServerV2, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewServerClient(clientFromProvider(p))
		},
		Kind: apitypes.KindNode,
		Name: "server",
		Codec: tfdriver.DataSourceCodecFuncs[apitypes.ServerV2]{
			SchemaFunc:  tfschema.GenSchemaServerV2DataSource,
			ToStateFunc: tfschema.CopyServerV2ToTerraform,
		},
		Identifier: tfdriver.PossiblyUnscopedScopeQualifiedNameIdentifierFromPath(
			tfdriver.ScopeQualifiedPath{
				Name:  path.Root("metadata").AtName("name"),
				Scope: path.Root("scope"),
			},
		),
	}
}

// NewSSHServerResourceType returns the SSH server resource type.
func NewSSHServerResourceType() tfdriver.ResourceType[apitypes.ServerV2, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.ResourceType[apitypes.ServerV2, tfdriver.ScopeQualifiedNameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[apitypes.ServerV2, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewServerClient(clientFromProvider(p))
		},
		Kind: apitypes.KindNode,
		Name: "server",
		Codec: tfdriver.ResourceCodecFuncs[apitypes.ServerV2]{
			SchemaFunc:   tfschema.GenSchemaServerV2Resource,
			ToStateFunc:  tfschema.CopyServerV2ToTerraform,
			FromPlanFunc: tfschema.CopyServerV2FromTerraform,
		},
		Normalizer: tfdriver.ResourceNormalizers[apitypes.ServerV2]{
			tfdriver.ForceKindFunc(func(server *apitypes.ServerV2) {
				server.Kind = apitypes.KindNode
			}),
			tfdriver.CheckAndSetDefaults[apitypes.ServerV2](),
		},
		Identifier: tfdriver.PossiblyUnscopedScopeQualifiedNameIdentifierPolicy(
			tfdriver.ScopeQualifiedPath{
				Name:  path.Root("metadata").AtName("name"),
				Scope: path.Root("scope"),
			},
			func(sv *apitypes.ServerV2) scopes.QualifiedName {
				return scopes.QualifiedName{
					Name:  sv.GetMetadata().Name,
					Scope: sv.GetScope()}
			}),
		ResourceRevision: func(st *apitypes.ServerV2) string {
			return st.GetMetadata().Revision
		},
	}
}
