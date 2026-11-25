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
	tftypes "github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// NewSAMLConnectorDataSourceType returns SAML connector data source type.
func NewSAMLConnectorDataSourceType() tfdriver.DataSourceType[types.SAMLConnectorV2, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[types.SAMLConnectorV2, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[types.SAMLConnectorV2, tfdriver.NameIdentifier] {
			return teleport.NewSAMLConnectorClient(clientFromProvider(p))
		},
		Kind: types.KindSAMLConnector,
		Name: "saml_connector",
		Codec: tfdriver.DataSourceCodecFuncs[types.SAMLConnectorV2]{
			SchemaFunc:  tfschema.GenSchemaSAMLConnectorV2DataSource,
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
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[types.SAMLConnectorV2, tfdriver.NameIdentifier] {
			return teleport.NewSAMLConnectorClient(clientFromProvider(p))
		},
		Kind: types.KindSAMLConnector,
		Name: "saml_connector",
		Codec: tfdriver.ResourceCodecFuncs[types.SAMLConnectorV2]{
			SchemaFunc:     tfschema.GenSchemaSAMLConnectorV2Resource,
			FromPlanFunc:   tfschema.CopySAMLConnectorV2FromTerraform,
			ToStateFunc:    tfschema.CopySAMLConnectorV2ToTerraform,
			ModifyPlanFunc: modifySAMLConnectorPlan,
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

// modifySAMLConnectorPlan modifies the SAML connector plan.
// It sets `spec.attributes_to_roles` from the config so that omitting it clears existing mappings instead of keeping them.
func modifySAMLConnectorPlan(ctx context.Context, config, plan tftypes.Object) (tftypes.Object, diag.Diagnostics) {
	val := new(types.SAMLConnectorV2)

	diags := tfschema.CopySAMLConnectorV2FromTerraform(ctx, config, val)
	if diags.HasError() {
		return plan, diags
	}

	const preserveUnknown = true
	config, diags = tfschema.CopySAMLConnectorV2ToTerraformPreserveUnknown(ctx, val, &config, preserveUnknown)
	if diags.HasError() {
		return plan, diags
	}

	planSpec, planOK := plan.Attributes()["spec"].(tftypes.Object)
	if !planOK || planSpec.IsNull() || planSpec.IsUnknown() {
		return plan, nil
	}

	configSpec, configOK := config.Attributes()["spec"].(tftypes.Object)
	if !configOK || configSpec.IsNull() || configSpec.IsUnknown() {
		return plan, nil
	}

	specAttrs := planSpec.Attributes()
	specAttrs["attributes_to_roles"] = configSpec.Attributes()["attributes_to_roles"]
	spec, d := tftypes.ObjectValue(planSpec.AttributeTypes(ctx), specAttrs)
	diags.Append(d...)
	if diags.HasError() {
		return plan, diags
	}

	attrs := plan.Attributes()
	attrs["spec"] = spec

	return tftypes.ObjectValue(plan.AttributeTypes(ctx), attrs)
}
