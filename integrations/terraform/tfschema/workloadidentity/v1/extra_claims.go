/*
Copyright 2026 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"context"
	"fmt"

	datasource_schema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	resource_schema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"

	"github.com/gravitational/teleport/integrations/terraform/tfschema/resource153"
)

// See the extra_claims exclusion in protoc-gen-terraform-workloadidentity.yaml.
const (
	extraClaimsJSONAttr        = "extra_claims_json"
	extraClaimsJSONDescription = "Additional claims that will be added to the JWT."
)

var (
	jwtPath             = []string{"spec", "spiffe", "jwt"}
	extraClaimsJSONPath = path.Root("spec").AtName("spiffe").AtName("jwt").AtName(extraClaimsJSONAttr)
)

// GenSchemaWorkloadIdentityResourceWithExtraClaims returns the WorkloadIdentity
// resource schema including extra_claims_json.
func GenSchemaWorkloadIdentityResourceWithExtraClaims(ctx context.Context) (resource_schema.Schema, diag.Diagnostics) {
	schema, diags := GenSchemaWorkloadIdentityResource(ctx)
	if diags.HasError() {
		return schema, diags
	}
	attrs := schema.Attributes
	for _, name := range jwtPath {
		attrs = attrs[name].(resource_schema.SingleNestedAttribute).Attributes
	}
	attr := attrs[extraClaimsJSONAttr].(resource_schema.StringAttribute)
	attr.Description = extraClaimsJSONDescription
	attrs[extraClaimsJSONAttr] = attr
	return schema, diags
}

// GenSchemaWorkloadIdentityDataSourceWithExtraClaims returns the
// WorkloadIdentity data source schema including extra_claims_json.
func GenSchemaWorkloadIdentityDataSourceWithExtraClaims(ctx context.Context) (datasource_schema.Schema, diag.Diagnostics) {
	schema, diags := GenSchemaWorkloadIdentityDataSource(ctx)
	if diags.HasError() {
		return schema, diags
	}
	attrs := schema.Attributes
	for _, name := range jwtPath {
		attrs = attrs[name].(datasource_schema.SingleNestedAttribute).Attributes
	}
	attr := attrs[extraClaimsJSONAttr].(datasource_schema.StringAttribute)
	attr.Description = extraClaimsJSONDescription
	attrs[extraClaimsJSONAttr] = attr
	return schema, diags
}

// CopyWorkloadIdentityFromTerraformWithExtraClaims is
// CopyWorkloadIdentityFromTerraform including extra_claims_json.
func CopyWorkloadIdentityFromTerraformWithExtraClaims(ctx context.Context, tf types.Object, obj *workloadidentityv1.WorkloadIdentity) diag.Diagnostics {
	diags := CopyWorkloadIdentityFromTerraform(ctx, tf, obj)
	if diags.HasError() {
		return diags
	}
	tfJWT, ok := jwtObject(tf)
	jwt := obj.GetSpec().GetSpiffe().GetJwt()
	if !ok || jwt == nil {
		return diags
	}
	value, ok := tfJWT.Attributes()[extraClaimsJSONAttr].(types.String)
	if !ok {
		diags.AddError("Error reading from Terraform object", fmt.Sprintf("Can not convert %T to String", tfJWT.Attributes()[extraClaimsJSONAttr]))
		return diags
	}
	claims, err := resource153.StructFromJSON(value)
	if err != nil {
		diags.AddAttributeError(extraClaimsJSONPath, "Invalid JSON object", err.Error())
		return diags
	}
	jwt.ExtraClaims = claims
	return diags
}

// CopyWorkloadIdentityToTerraformWithExtraClaims is
// CopyWorkloadIdentityToTerraform including extra_claims_json.
func CopyWorkloadIdentityToTerraformWithExtraClaims(ctx context.Context, obj *workloadidentityv1.WorkloadIdentity, tf *types.Object) (types.Object, diag.Diagnostics) {
	result, diags := CopyWorkloadIdentityToTerraform(ctx, obj, tf)
	if diags.HasError() {
		return result, diags
	}
	tfJWT, ok := jwtObject(result)
	if !ok {
		return result, diags
	}
	value, err := resource153.JSONFromStruct(obj.GetSpec().GetSpiffe().GetJwt().GetExtraClaims(), tfJWT.Attributes()[extraClaimsJSONAttr])
	if err != nil {
		diags.AddAttributeError(extraClaimsJSONPath, "Error converting extra claims", err.Error())
		return result, diags
	}
	result, d := withExtraClaimsJSON(ctx, result, jwtPath, value)
	diags.Append(d...)
	return result, diags
}

// ModifyWorkloadIdentityPlanWithExtraClaims wraps modifyPlan, which rebuilds
// spec from config, so extra_claims_json keeps the value chosen by its plan
// modifier.
func ModifyWorkloadIdentityPlanWithExtraClaims(modifyPlan func(context.Context, types.Object, types.Object) (types.Object, diag.Diagnostics)) func(context.Context, types.Object, types.Object) (types.Object, diag.Diagnostics) {
	return func(ctx context.Context, config, plan types.Object) (types.Object, diag.Diagnostics) {
		result, diags := modifyPlan(ctx, config, plan)
		if diags.HasError() {
			return result, diags
		}
		planJWT, ok := jwtObject(plan)
		if !ok {
			return result, diags
		}
		if _, ok := jwtObject(result); !ok {
			return result, diags
		}
		value, ok := planJWT.Attributes()[extraClaimsJSONAttr].(types.String)
		if !ok {
			return result, diags
		}
		result, d := withExtraClaimsJSON(ctx, result, jwtPath, value)
		diags.Append(d...)
		return result, diags
	}
}

func jwtObject(tf types.Object) (types.Object, bool) {
	obj := tf
	for _, name := range jwtPath {
		next, ok := obj.Attributes()[name].(types.Object)
		if !ok || next.IsNull() || next.IsUnknown() {
			return types.Object{}, false
		}
		obj = next
	}
	return obj, true
}

// Objects are immutable, so each level is rebuilt.
func withExtraClaimsJSON(ctx context.Context, obj types.Object, names []string, value types.String) (types.Object, diag.Diagnostics) {
	attrs := obj.Attributes()
	if len(names) == 0 {
		attrs[extraClaimsJSONAttr] = value
	} else {
		child, diags := withExtraClaimsJSON(ctx, attrs[names[0]].(types.Object), names[1:], value)
		if diags.HasError() {
			return obj, diags
		}
		attrs[names[0]] = child
	}
	return types.ObjectValue(obj.AttributeTypes(ctx), attrs)
}
