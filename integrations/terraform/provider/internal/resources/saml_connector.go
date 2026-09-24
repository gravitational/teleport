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

// NewSAMLConnectorDataSourceType returns SAML connector data source type.
func NewSAMLConnectorDataSourceType() tfdriver.DataSourceType[types.SAMLConnectorV2, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[types.SAMLConnectorV2, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[types.SAMLConnectorV2, tfdriver.NameIdentifier] {
			return teleport.NewSAMLConnectorClient(clientFromProvider(p))
		},
		Kind: types.KindSAMLConnector,
		Codec: tfdriver.DataSourceCodecFuncs[types.SAMLConnectorV2]{
			SchemaFunc:  tfschema.GenSchemaSAMLConnectorV2,
			ToStateFunc: tfschema.CopySAMLConnectorV2ToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(
			path.Root("metadata").AtName("name"),
		),
	}
}

// NewSAMLConnectorResourceType returns SAML connector resource type.
func NewSAMLConnectorResourceType() tfdriver.ResourceType[types.SAMLConnectorV2, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[types.SAMLConnectorV2, tfdriver.NameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[types.SAMLConnectorV2, tfdriver.NameIdentifier] {
			return teleport.NewSAMLConnectorClient(clientFromProvider(p))
		},
		Kind: types.KindSAMLConnector,
		Codec: tfdriver.ResourceCodecFuncs[types.SAMLConnectorV2]{
			SchemaFunc:   tfschema.GenSchemaSAMLConnectorV2,
			FromPlanFunc: tfschema.CopySAMLConnectorV2FromTerraform,
			ToStateFunc:  tfschema.CopySAMLConnectorV2ToTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[types.SAMLConnectorV2](),
		Identifier: tfdriver.NameIdentifierPolicy(
			path.Root("metadata").AtName("name"),
			func(connector *types.SAMLConnectorV2) string {
				return connector.GetMetadata().Name
			},
		),
		ResourceRevision: func(connector *types.SAMLConnectorV2) string {
			return connector.GetMetadata().Revision
		},
	}
}
