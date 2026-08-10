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

// NewAccessListMemberDataSourceType returns the app data source type.
func NewAccessListMemberDataSourceType() tfdriver.DataSourceType[accesslist.AccessListMember, tfdriver.ScopeQualifiedCompositeIdentifier] {
	return tfdriver.DataSourceType[accesslist.AccessListMember, tfdriver.ScopeQualifiedCompositeIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[accesslist.AccessListMember, tfdriver.ScopeQualifiedCompositeIdentifier] {
			return teleport.NewAccessListMemberClient(clientFromProvider(p))
		},
		Kind: apitypes.KindAccessListMember,
		Codec: tfdriver.DataSourceCodecFuncs[accesslist.AccessListMember]{
			SchemaFunc: schemav1.GenSchemaMember,
			ToStateFunc: func(ctx context.Context, alm *accesslist.AccessListMember, o *types.Object) diag.Diagnostics {
				return schemav1.CopyMemberToTerraform(ctx, convertv1.ToMemberProto(alm), o)
			},
		},
		Identifier: tfdriver.ScopeQualifiedCompositeIdentifierFromPath(
			path.Root("spec").AtName("access_list"),
			path.Root("header").AtName("metadata").AtName("name"),
		),
	}
}

// NewAccessListMemberResourceType returns the app resource type.
func NewAccessListMemberResourceType() tfdriver.ResourceType[accesslist.AccessListMember, tfdriver.ScopeQualifiedCompositeIdentifier] {
	return tfdriver.ResourceType[accesslist.AccessListMember, tfdriver.ScopeQualifiedCompositeIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[accesslist.AccessListMember, tfdriver.ScopeQualifiedCompositeIdentifier] {
			return teleport.NewAccessListMemberClient(clientFromProvider(p))
		},
		Kind: apitypes.KindAccessListMember,
		Codec: tfdriver.ResourceCodecFuncs[accesslist.AccessListMember]{
			SchemaFunc: schemav1.GenSchemaMember,
			ToStateFunc: func(ctx context.Context, alm *accesslist.AccessListMember, o *types.Object) diag.Diagnostics {
				return schemav1.CopyMemberToTerraform(ctx, convertv1.ToMemberProto(alm), o)

			},
			FromPlanFunc: func(ctx context.Context, o types.Object, alm *accesslist.AccessListMember) diag.Diagnostics {
				protoMember := new(accesslistv1.Member)
				diags := schemav1.CopyMemberFromTerraform(ctx, o, protoMember)
				if diags.HasError() {
					return diags
				}

				converted, err := convertv1.FromMemberProto(protoMember)
				if err != nil {
					diags.AddError("Error converting access list member", err.Error())
					return diags
				}

				*alm = *converted
				return diags

			},
		},
		Normalizer: tfdriver.CheckAndSetDefaults[accesslist.AccessListMember](),
		Identifier: tfdriver.ScopeQualifiedCompositeIdentifierPolicy(
			path.Root("spec").AtName("access_list"),
			path.Root("header").AtName("metadata").AtName("name"),
			func(av *accesslist.AccessListMember) (string, string) {
				return av.Spec.AccessList, av.GetMetadata().Name
			}),
		ResourceRevision: func(st *accesslist.AccessListMember) string {
			return st.GetMetadata().Revision
		},
	}
}
