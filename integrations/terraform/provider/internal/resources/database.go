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

	apitypes "github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// NewDatabaseDataSourceType returns the database data source type.
func NewDatabaseDataSourceType() tfdriver.DataSourceType[apitypes.DatabaseV3, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[apitypes.DatabaseV3, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[apitypes.DatabaseV3, tfdriver.NameIdentifier] {
			return teleport.NewDatabaseClient(clientFromProvider(p))
		},
		Kind: apitypes.KindDatabase,
		Name: "database",
		Codec: tfdriver.DataSourceCodecFuncs[apitypes.DatabaseV3]{
			SchemaFunc:  tfschema.GenSchemaDatabaseV3,
			ToStateFunc: tfschema.CopyDatabaseV3ToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewDatabaseResourceType returns the database resource type.
func NewDatabaseResourceType() tfdriver.ResourceType[apitypes.DatabaseV3, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[apitypes.DatabaseV3, tfdriver.NameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[apitypes.DatabaseV3, tfdriver.NameIdentifier] {
			return teleport.NewDatabaseClient(clientFromProvider(p))
		},
		Kind: apitypes.KindDatabase,
		Name: "database",
		Codec: tfdriver.ResourceCodecFuncs[apitypes.DatabaseV3]{
			SchemaFunc:   tfschema.GenSchemaDatabaseV3,
			ToStateFunc:  tfschema.CopyDatabaseV3ToTerraform,
			FromPlanFunc: tfschema.CopyDatabaseV3FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[apitypes.DatabaseV3](),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(database *apitypes.DatabaseV3) string {
			return database.GetMetadata().Name
		}),
		ResourceRevision: func(st *apitypes.DatabaseV3) string {
			return st.GetMetadata().Revision
		},
	}
}
