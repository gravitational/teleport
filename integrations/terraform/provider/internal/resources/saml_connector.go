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
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	apitypes "github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// NewSAMLConnectorDataSourceType returns SAML connector data source type.
func NewSAMLConnectorDataSourceType() tfdriver.DataSourceType[apitypes.SAMLConnectorV2, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[apitypes.SAMLConnectorV2, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[apitypes.SAMLConnectorV2, tfdriver.NameIdentifier] {
			return teleport.NewSAMLConnectorClient(clientFromProvider(p))
		},
		Kind: apitypes.KindSAMLConnector,
		Codec: tfdriver.DataSourceCodecFuncs[apitypes.SAMLConnectorV2]{
			SchemaFunc:  tfschema.GenSchemaSAMLConnectorV2,
			ToStateFunc: tfschema.CopySAMLConnectorV2ToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(
			path.Root("metadata").AtName("name"),
		),
	}
}

// NewSAMLConnectorResourceType returns SAML connector resource type.
func NewSAMLConnectorResourceType() tfdriver.ResourceType[apitypes.SAMLConnectorV2, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[apitypes.SAMLConnectorV2, tfdriver.NameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[apitypes.SAMLConnectorV2, tfdriver.NameIdentifier] {
			return teleport.NewSAMLConnectorClient(clientFromProvider(p))
		},
		Kind: apitypes.KindSAMLConnector,
		Codec: tfdriver.ResourceCodecFuncs[apitypes.SAMLConnectorV2]{
			SchemaFunc:     tfschema.GenSchemaSAMLConnectorV2,
			FromConfigFunc: tfschema.CopySAMLConnectorV2FromTerraform,
			FromPlanFunc:   tfschema.CopySAMLConnectorV2FromTerraform,
			ToConfigFunc: func(ctx context.Context, samlConnector *apitypes.SAMLConnectorV2, o *types.Object) diag.Diagnostics {
				const preserveUnknown = true
				return tfschema.CopySAMLConnectorV2ToTerraformPreserveUnknown(ctx, samlConnector, o, preserveUnknown)
			},
			ToStateFunc: tfschema.CopySAMLConnectorV2ToTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[apitypes.SAMLConnectorV2](),
		Identifier: tfdriver.NameIdentifierPolicy(
			path.Root("metadata").AtName("name"),
			func(connector *apitypes.SAMLConnectorV2) string {
				return connector.GetMetadata().Name
			},
		),
		ResourceRevision: func(connector *apitypes.SAMLConnectorV2) string {
			return connector.GetMetadata().Revision
		},
	}
}
