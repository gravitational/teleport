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

	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// NewOktaImportRuleDataSourceType returns the Okta import rule data source type.
func NewOktaImportRuleDataSourceType() tfdriver.DataSourceType[types.OktaImportRuleV1, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[types.OktaImportRuleV1, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[types.OktaImportRuleV1, tfdriver.NameIdentifier] {
			return teleport.NewOktaImportRuleClient(clientFromProvider(p))
		},
		Kind: types.KindOktaImportRule,
		Name: types.KindOktaImportRule,
		Codec: tfdriver.DataSourceCodecFuncs[types.OktaImportRuleV1]{
			SchemaFunc:  tfschema.GenSchemaOktaImportRuleV1DataSource,
			ToStateFunc: tfschema.CopyOktaImportRuleV1ToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewOktaImportRuleResourceType returns the Okta import rule resource type.
func NewOktaImportRuleResourceType() tfdriver.ResourceType[types.OktaImportRuleV1, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[types.OktaImportRuleV1, tfdriver.NameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[types.OktaImportRuleV1, tfdriver.NameIdentifier] {
			return teleport.NewOktaImportRuleClient(clientFromProvider(p))
		},
		Kind: types.KindOktaImportRule,
		Name: types.KindOktaImportRule,
		Codec: tfdriver.ResourceCodecFuncs[types.OktaImportRuleV1]{
			SchemaFunc:   tfschema.GenSchemaOktaImportRuleV1Resource,
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
