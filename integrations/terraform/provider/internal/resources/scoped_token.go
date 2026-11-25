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

	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/scopes"
	apitypes "github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/scopes/joining/v1"
)

// NewScopedTokenDataSourceType returns the scoped token data source type.
func NewScopedTokenDataSourceType() tfdriver.DataSourceType[joiningv1.ScopedToken, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.DataSourceType[joiningv1.ScopedToken, tfdriver.ScopeQualifiedNameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[joiningv1.ScopedToken, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewScopedTokenClient(clientFromProvider(p))
		},
		Identifier: tfdriver.ScopeQualifiedNameIdentifierFromPath(
			tfdriver.ScopeQualifiedPath{
				Name:  path.Root("metadata").AtName("name"),
				Scope: path.Root("scope"),
			}),
		Kind: apitypes.KindScopedToken,
		Name: apitypes.KindScopedToken,
		Codec: tfdriver.DataSourceCodecFuncs[joiningv1.ScopedToken]{
			SchemaFunc:  schemav1.GenSchemaScopedTokenDataSource,
			ToStateFunc: schemav1.CopyScopedTokenToTerraform,
		},
	}
}

// NewScopedTokenResourceType returns the scoped token resource type.
func NewScopedTokenResourceType() tfdriver.ResourceType[joiningv1.ScopedToken, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.ResourceType[joiningv1.ScopedToken, tfdriver.ScopeQualifiedNameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[joiningv1.ScopedToken, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewScopedTokenClient(clientFromProvider(p))
		},
		Kind: apitypes.KindScopedToken,
		Name: apitypes.KindScopedToken,
		Codec: tfdriver.ResourceCodecFuncs[joiningv1.ScopedToken]{
			SchemaFunc:   schemav1.GenSchemaScopedTokenResource,
			ToStateFunc:  schemav1.CopyScopedTokenToTerraform,
			FromPlanFunc: schemav1.CopyScopedTokenFromTerraform,
			ModifyPlanFunc: tfdriver.NullToZeroModifyPlan(
				schemav1.CopyScopedTokenFromTerraform,
				schemav1.CopyScopedTokenToTerraformPreserveUnknown,
			),
		},
		Normalizer: tfdriver.ForceKind[joiningv1.ScopedToken](apitypes.KindScopedToken),
		Identifier: tfdriver.ScopeQualifiedNameIdentifierPolicy(
			tfdriver.ScopeQualifiedPath{
				Name:  path.Root("metadata").AtName("name"),
				Scope: path.Root("scope"),
			},
			func(st *joiningv1.ScopedToken) scopes.QualifiedName {
				return scopes.QualifiedName{
					Name:  st.GetMetadata().GetName(),
					Scope: st.GetScope(),
				}
			},
		),
		ResourceRevision: func(st *joiningv1.ScopedToken) string {
			return st.GetMetadata().GetRevision()
		},
	}
}
