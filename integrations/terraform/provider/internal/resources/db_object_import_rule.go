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
			SchemaFunc:  schemav1.GenSchemaDatabaseObjectImportRule,
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
			SchemaFunc:   schemav1.GenSchemaDatabaseObjectImportRule,
			ToStateFunc:  schemav1.CopyDatabaseObjectImportRuleToTerraform,
			FromPlanFunc: schemav1.CopyDatabaseObjectImportRuleFromTerraform,
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
