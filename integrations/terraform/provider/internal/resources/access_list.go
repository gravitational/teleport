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
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	accesslistv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1"
	apitypes "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	convertv1 "github.com/gravitational/teleport/api/types/accesslist/convert/v1"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/accesslist/v1"
)

// NewAccessListDataSourceType returns the app data source type.
func NewAccessListDataSourceType() tfdriver.DataSourceType[accesslist.AccessList, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.DataSourceType[accesslist.AccessList, tfdriver.ScopeQualifiedNameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[accesslist.AccessList, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewAccessListClient(clientFromProvider(p))
		},
		Kind: apitypes.KindAccessList,
		Codec: tfdriver.DataSourceCodecFuncs[accesslist.AccessList]{
			SchemaFunc: schemav1.GenSchemaAccessList,
			ToStateFunc: func(ctx context.Context, al *accesslist.AccessList, o *types.Object) diag.Diagnostics {
				return schemav1.CopyAccessListToTerraform(ctx, convertv1.ToProto(al), o)
			},
		},
		Identifier: tfdriver.PossiblyUnscopedScopeQualifiedNameIdentifierFromPath(
			path.Root("header").AtName("metadata").AtName("name"),
			path.Root("scope"),
		),
	}
}

// NewAccessListResourceType returns the app resource type.
func NewAccessListResourceType() tfdriver.ResourceType[accesslist.AccessList, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.ResourceType[accesslist.AccessList, tfdriver.ScopeQualifiedNameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[accesslist.AccessList, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewAccessListClient(clientFromProvider(p))
		},
		Kind: apitypes.KindAccessList,
		Codec: tfdriver.ResourceCodecFuncs[accesslist.AccessList]{
			SchemaFunc: schemav1.GenSchemaAccessList,
			ToStateFunc: func(ctx context.Context, al *accesslist.AccessList, o *types.Object) diag.Diagnostics {

				return schemav1.CopyAccessListToTerraform(ctx, convertv1.ToProto(al), o)
			},
			FromPlanFunc: func(ctx context.Context, o types.Object, al *accesslist.AccessList) diag.Diagnostics {
				protoACL := new(accesslistv1.AccessList)
				diags := schemav1.CopyAccessListFromTerraform(ctx, o, protoACL)
				if diags.HasError() {
					return diags
				}

				converted, err := convertv1.FromProto(protoACL)
				if err != nil {
					diags.AddError("Error converting access list", err.Error())
					return diags
				}

				*al = *converted
				return diags

			},
		},
		Normalizer: tfdriver.CheckAndSetDefaults[accesslist.AccessList](),
		Identifier: tfdriver.PossiblyUnscopedScopeQualifiedNameIdentifierPolicy(
			path.Root("header").AtName("metadata").AtName("name"),
			path.Root("scope"),
			func(av *accesslist.AccessList) (string, string) {
				return av.GetMetadata().Name, av.GetScope()
			},
		),
		ResourceRevision: func(st *accesslist.AccessList) string {
			return st.GetMetadata().Revision
		},
	}
}
