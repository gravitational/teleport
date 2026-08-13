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

	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// NewOktaImportRuleDataSourceType returns the Okta import rule data source type.
func NewOktaImportRuleDataSourceType() tfdriver.DataSourceType[types.OktaImportRuleV1, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[types.OktaImportRuleV1, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[types.OktaImportRuleV1, tfdriver.NameIdentifier] {
			return teleport.NewOktaImportRuleClient(clientFromProvider(p))
		},
		Kind: types.KindOktaImportRule,
		Codec: tfdriver.DataSourceCodecFuncs[types.OktaImportRuleV1]{
			SchemaFunc:  tfschema.GenSchemaOktaImportRuleV1,
			ToStateFunc: tfschema.CopyOktaImportRuleV1ToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewOktaImportRuleResourceType returns the Okta import rule resource type.
func NewOktaImportRuleResourceType() tfdriver.ResourceType[types.OktaImportRuleV1, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[types.OktaImportRuleV1, tfdriver.NameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[types.OktaImportRuleV1, tfdriver.NameIdentifier] {
			return teleport.NewOktaImportRuleClient(clientFromProvider(p))
		},
		Kind: types.KindOktaImportRule,
		Codec: tfdriver.ResourceCodecFuncs[types.OktaImportRuleV1]{
			SchemaFunc:   tfschema.GenSchemaOktaImportRuleV1,
			ToStateFunc:  tfschema.CopyOktaImportRuleV1ToTerraform,
			FromPlanFunc: tfschema.CopyOktaImportRuleV1FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[types.OktaImportRuleV1](),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(importRule *types.OktaImportRuleV1) string {
			return importRule.GetMetadata().Name
		}),
		ResourceRevision: func(st *types.OktaImportRuleV1) string {
			return st.GetMetadata().Revision
		},
	}
}
