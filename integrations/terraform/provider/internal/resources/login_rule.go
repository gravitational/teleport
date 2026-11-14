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
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/types"

	loginrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	apitypes "github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/loginrule/v1"
)

// NewLoginRuleDataSourceType returns the login rule data source type.
func NewLoginRuleDataSourceType() tfdriver.DataSourceType[loginrulev1.LoginRule, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[loginrulev1.LoginRule, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[loginrulev1.LoginRule, tfdriver.NameIdentifier] {
			return teleport.NewLoginRuleClient(clientFromProvider(p))
		},
		Kind: apitypes.KindLoginRule,
		Name: apitypes.KindLoginRule,
		Codec: tfdriver.DataSourceCodecFuncs[loginrulev1.LoginRule]{
			SchemaFunc:  schemav1.GenSchemaLoginRuleDataSource,
			ToStateFunc: schemav1.CopyLoginRuleToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewLoginRuleResourceType returns the login rule resource type.
func NewLoginRuleResourceType() tfdriver.ResourceType[loginrulev1.LoginRule, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[loginrulev1.LoginRule, tfdriver.NameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[loginrulev1.LoginRule, tfdriver.NameIdentifier] {
			return teleport.NewLoginRuleClient(clientFromProvider(p))
		},
		Kind: apitypes.KindLoginRule,
		Name: apitypes.KindLoginRule,
		Codec: tfdriver.ResourceCodecFuncs[loginrulev1.LoginRule]{
			SchemaFunc:     schemav1.GenSchemaLoginRuleResource,
			ToStateFunc:    schemav1.CopyLoginRuleToTerraform,
			FromPlanFunc:   schemav1.CopyLoginRuleFromTerraform,
			ModifyPlanFunc: modifyLoginRulePlan,
		},
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(loginRule *loginrulev1.LoginRule) string {
			return loginRule.GetMetadata().GetName()
		}),
		ResourceRevision: func(st *loginrulev1.LoginRule) string {
			return st.GetMetadata().GetRevision()
		},
	}
}

// modifyLoginRulePlan modifies the login rule plan and sets zero values
// for spec attribute values. A custom implementation is used because login_rule
// is atypical and does not contain a `spec` attribute.
func modifyLoginRulePlan(ctx context.Context, config, plan types.Object) (types.Object, diag.Diagnostics) {
	val := new(loginrulev1.LoginRule)

	diags := schemav1.CopyLoginRuleFromTerraform(ctx, config, val)
	if diags.HasError() {
		return plan, diags
	}

	const preserveUnknown = true
	config, diags = schemav1.CopyLoginRuleToTerraformPreserveUnknown(ctx, val, &config, preserveUnknown)
	if diags.HasError() {
		return plan, diags
	}

	attrs := plan.Attributes()
	attributes := []string{"priority", "traits_map", "traits_expression"}
	for _, attribute := range attributes {
		if value, exists := config.Attributes()[attribute]; exists {
			attrs[attribute] = value
		}
	}

	return types.ObjectValue(plan.AttributeTypes(ctx), attrs)
}
