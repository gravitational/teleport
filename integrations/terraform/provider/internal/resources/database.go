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
			SchemaFunc:  tfschema.GenSchemaDatabaseV3DataSource,
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
			SchemaFunc:   tfschema.GenSchemaDatabaseV3Resource,
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
