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

	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	apitypes "github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/scopes/joining/v1"
)

// NewScopedTokenDataSourceType returns the scoped token data source type.
func NewScopedTokenDataSourceType() tfdriver.DataSourceType[joiningv1.ScopedToken, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.DataSourceType[joiningv1.ScopedToken, tfdriver.ScopeQualifiedNameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[joiningv1.ScopedToken, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewScopedTokenClient(clientFromProvider(p))
		},
		Identifier: tfdriver.ScopeQualifiedNameIdentifierFromPath(
			path.Root("metadata").AtName("name"),
			path.Root("scope"),
		),
		Kind: apitypes.KindScopedToken,
		Codec: tfdriver.DataSourceCodecFuncs[joiningv1.ScopedToken]{
			SchemaFunc:  schemav1.GenSchemaScopedToken,
			ToStateFunc: schemav1.CopyScopedTokenToTerraform,
		},
	}
}

// NewScopedTokenResourceType returns the scoped token resource type.
func NewScopedTokenResourceType() tfdriver.ResourceType[joiningv1.ScopedToken, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.ResourceType[joiningv1.ScopedToken, tfdriver.ScopeQualifiedNameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[joiningv1.ScopedToken, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewScopedTokenClient(clientFromProvider(p))
		},
		Kind: apitypes.KindScopedToken,
		Codec: tfdriver.ResourceCodecFuncs[joiningv1.ScopedToken]{
			SchemaFunc:   schemav1.GenSchemaScopedToken,
			ToStateFunc:  schemav1.CopyScopedTokenToTerraform,
			FromPlanFunc: schemav1.CopyScopedTokenFromTerraform,
		},
		Normalizer: tfdriver.ForceKind[joiningv1.ScopedToken](apitypes.KindScopedToken),
		Identifier: tfdriver.ScopeQualifiedNameIdentifierPolicy(
			path.Root("metadata").AtName("name"),
			path.Root("scope"),
			func(st *joiningv1.ScopedToken) (string, string) {
				return st.GetMetadata().GetName(), st.GetScope()
			},
		),
		ResourceRevision: func(st *joiningv1.ScopedToken) string {
			return st.GetMetadata().GetRevision()
		},
	}
}
