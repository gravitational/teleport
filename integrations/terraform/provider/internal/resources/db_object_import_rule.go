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

	dbobjectimportrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/dbobjectimportrule/v1"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/dbobjectimportrule/v1"
)

// NewDatabaseObjectImportRuleDataSourceType returns the database object import rule data source type.
func NewDatabaseObjectImportRuleDataSourceType() tfdriver.DataSourceType[dbobjectimportrulev1.DatabaseObjectImportRule, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[dbobjectimportrulev1.DatabaseObjectImportRule, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[dbobjectimportrulev1.DatabaseObjectImportRule, tfdriver.NameIdentifier] {
			return teleport.NewDatabaseObjectImportRuleClient(clientFromProvider(p))
		},
		Kind: types.KindDatabaseObjectImportRule,
		Name: types.KindDatabaseObjectImportRule,
		Codec: tfdriver.DataSourceCodecFuncs[dbobjectimportrulev1.DatabaseObjectImportRule]{
			SchemaFunc:  schemav1.GenSchemaDatabaseObjectImportRuleDataSource,
			ToStateFunc: schemav1.CopyDatabaseObjectImportRuleToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewDatabaseObjectImportRuleResourceType returns the database object import rule resource type.
func NewDatabaseObjectImportRuleResourceType() tfdriver.ResourceType[dbobjectimportrulev1.DatabaseObjectImportRule, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[dbobjectimportrulev1.DatabaseObjectImportRule, tfdriver.NameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[dbobjectimportrulev1.DatabaseObjectImportRule, tfdriver.NameIdentifier] {
			return teleport.NewDatabaseObjectImportRuleClient(clientFromProvider(p))
		},
		Kind: types.KindDatabaseObjectImportRule,
		Name: types.KindDatabaseObjectImportRule,
		Codec: tfdriver.ResourceCodecFuncs[dbobjectimportrulev1.DatabaseObjectImportRule]{
			SchemaFunc:   schemav1.GenSchemaDatabaseObjectImportRuleResource,
			ToStateFunc:  schemav1.CopyDatabaseObjectImportRuleToTerraform,
			FromPlanFunc: schemav1.CopyDatabaseObjectImportRuleFromTerraform,
			ModifyPlanFunc: tfdriver.NullToZeroModifyPlan(
				schemav1.CopyDatabaseObjectImportRuleFromTerraform,
				schemav1.CopyDatabaseObjectImportRuleToTerraformPreserveUnknown,
			),
		},
		Normalizer: tfdriver.ForceKind[dbobjectimportrulev1.DatabaseObjectImportRule](types.KindDatabaseObjectImportRule),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(importRule *dbobjectimportrulev1.DatabaseObjectImportRule) string {
			return importRule.GetMetadata().GetName()
		}),
		ResourceRevision: func(st *dbobjectimportrulev1.DatabaseObjectImportRule) string {
			return st.GetMetadata().GetRevision()
		},
	}
}
