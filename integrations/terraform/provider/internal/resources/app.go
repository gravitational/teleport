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

// NewAppDataSourceType returns the app data source type.
func NewAppDataSourceType() tfdriver.DataSourceType[apitypes.AppV3, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.DataSourceType[apitypes.AppV3, tfdriver.ScopeQualifiedNameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[apitypes.AppV3, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewAppClient(clientFromProvider(p))
		},
		Kind: apitypes.KindApp,
		Name: apitypes.KindApp,
		Codec: tfdriver.DataSourceCodecFuncs[apitypes.AppV3]{
			SchemaFunc:  tfschema.GenSchemaAppV3DataSource,
			ToStateFunc: tfschema.CopyAppV3ToTerraform,
		},
		Identifier: tfdriver.PossiblyUnscopedScopeQualifiedNameIdentifierFromPath(
			tfdriver.ScopeQualifiedPath{
				Name:  path.Root("metadata").AtName("name"),
				Scope: path.Root("scope"),
			},
		),
	}
}

// NewAppResourceType returns the app resource type.
func NewAppResourceType() tfdriver.ResourceType[apitypes.AppV3, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.ResourceType[apitypes.AppV3, tfdriver.ScopeQualifiedNameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[apitypes.AppV3, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewAppClient(clientFromProvider(p))
		},
		Kind: apitypes.KindApp,
		Name: apitypes.KindApp,
		Codec: tfdriver.ResourceCodecFuncs[apitypes.AppV3]{
			SchemaFunc:   tfschema.GenSchemaAppV3Resource,
			ToStateFunc:  tfschema.CopyAppV3ToTerraform,
			FromPlanFunc: tfschema.CopyAppV3FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[apitypes.AppV3](),
		Identifier: tfdriver.PossiblyUnscopedScopeQualifiedNameIdentifierPolicy(
			tfdriver.ScopeQualifiedPath{
				Name:  path.Root("metadata").AtName("name"),
				Scope: path.Root("scope"),
			},
			func(av *apitypes.AppV3) scopes.QualifiedName {
				return scopes.QualifiedName{
					Name:  av.GetMetadata().Name,
					Scope: av.GetScope(),
				}
			}),
		ResourceRevision: func(st *apitypes.AppV3) string {
			return st.GetMetadata().Revision
		},
	}
}
