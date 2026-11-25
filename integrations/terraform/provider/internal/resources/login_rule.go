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
