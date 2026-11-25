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
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
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
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[accesslist.AccessListMember, tfdriver.ScopeQualifiedCompositeIdentifier] {
			return teleport.NewAccessListMemberClient(clientFromProvider(p))
		},
		Kind: apitypes.KindAccessListMember,
		Name: apitypes.KindAccessListMember,
		Codec: tfdriver.DataSourceCodecFuncs[accesslist.AccessListMember]{
			SchemaFunc: schemav1.GenSchemaMemberDataSource,
			ToStateFunc: func(ctx context.Context, alm *accesslist.AccessListMember, o *types.Object) (types.Object, diag.Diagnostics) {
				return schemav1.CopyMemberToTerraform(ctx, convertv1.ToMemberProto(alm), o)
			},
		},
		Identifier: tfdriver.ScopeQualifiedCompositeIdentifierFromPath(tfdriver.CompositeIdentifierPath{
			Prefix: path.Root("spec").AtName("access_list"),
			Name:   path.Root("header").AtName("metadata").AtName("name"),
		},
		),
	}
}

// NewAccessListMemberResourceType returns the app resource type.
func NewAccessListMemberResourceType() tfdriver.ResourceType[accesslist.AccessListMember, tfdriver.ScopeQualifiedCompositeIdentifier] {
	return tfdriver.ResourceType[accesslist.AccessListMember, tfdriver.ScopeQualifiedCompositeIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[accesslist.AccessListMember, tfdriver.ScopeQualifiedCompositeIdentifier] {
			return teleport.NewAccessListMemberClient(clientFromProvider(p))
		},
		Kind: apitypes.KindAccessListMember,
		Name: apitypes.KindAccessListMember,
		Codec: tfdriver.ResourceCodecFuncs[accesslist.AccessListMember]{
			SchemaFunc: schemav1.GenSchemaMemberResource,
			ToStateFunc: func(ctx context.Context, alm *accesslist.AccessListMember, o *types.Object) (types.Object, diag.Diagnostics) {
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
			tfdriver.CompositeIdentifierPath{
				Prefix: path.Root("spec").AtName("access_list"),
				Name:   path.Root("header").AtName("metadata").AtName("name"),
			},
			func(av *accesslist.AccessListMember) tfdriver.CompositeIdentifier {
				return tfdriver.CompositeIdentifier{
					Prefix: av.Spec.AccessList,
					Name:   av.GetMetadata().Name,
				}
			}),
		ResourceRevision: func(st *accesslist.AccessListMember) string {
			return st.GetMetadata().Revision
		},
	}
}
