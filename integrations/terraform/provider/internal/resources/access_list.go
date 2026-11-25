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
	"github.com/gravitational/teleport/api/scopes"
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
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[accesslist.AccessList, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewAccessListClient(clientFromProvider(p))
		},
		Kind: apitypes.KindAccessList,
		Name: apitypes.KindAccessList,
		Codec: tfdriver.DataSourceCodecFuncs[accesslist.AccessList]{
			SchemaFunc: schemav1.GenSchemaAccessListDataSource,
			ToStateFunc: func(ctx context.Context, al *accesslist.AccessList, o *types.Object) (types.Object, diag.Diagnostics) {
				return schemav1.CopyAccessListToTerraform(ctx, convertv1.ToProto(al), o)
			},
		},
		Identifier: tfdriver.PossiblyUnscopedScopeQualifiedNameIdentifierFromPath(
			tfdriver.ScopeQualifiedPath{
				Name:  path.Root("header").AtName("metadata").AtName("name"),
				Scope: path.Root("scope"),
			},
		),
	}
}

// NewAccessListResourceType returns the app resource type.
func NewAccessListResourceType() tfdriver.ResourceType[accesslist.AccessList, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.ResourceType[accesslist.AccessList, tfdriver.ScopeQualifiedNameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[accesslist.AccessList, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewAccessListClient(clientFromProvider(p))
		},
		Kind: apitypes.KindAccessList,
		Name: apitypes.KindAccessList,
		Codec: tfdriver.ResourceCodecFuncs[accesslist.AccessList]{
			SchemaFunc: schemav1.GenSchemaAccessListResource,
			ToStateFunc: func(ctx context.Context, al *accesslist.AccessList, o *types.Object) (types.Object, diag.Diagnostics) {

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
			tfdriver.ScopeQualifiedPath{
				Name:  path.Root("header").AtName("metadata").AtName("name"),
				Scope: path.Root("scope"),
			},
			func(av *accesslist.AccessList) scopes.QualifiedName {
				return scopes.QualifiedName{
					Name:  av.GetMetadata().Name,
					Scope: av.GetScope(),
				}
			},
		),
		ResourceRevision: func(st *accesslist.AccessList) string {
			return st.GetMetadata().Revision
		},
	}
}
