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

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	loginrulev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"

	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/loginrule/v1"
)

// ModifyPlan modifies the planned value, normalizing null values.
func (r resourceTeleportLoginRule) ModifyPlan(ctx context.Context, req tfsdk.ModifyResourcePlanRequest, resp *tfsdk.ModifyResourcePlanResponse) {
	// If the entire plan is null, the resource is planned for destruction.
	if req.Plan.Raw.IsNull() {
		return
	}

	// If the state is null, the resource is being created. No need to modify plan.
	if req.State.Raw.IsNull() {
		return
	}

	var config types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	loginRule := &loginrulev1.LoginRule{}
	resp.Diagnostics.Append(schemav1.CopyLoginRuleFromTerraform(ctx, config, loginRule)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(schemav1.CopyLoginRuleToTerraform(ctx, loginRule, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan types.Object
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Attrs["priority"] = config.Attrs["priority"]
	plan.Attrs["traits_expression"] = config.Attrs["traits_expression"]
	plan.Attrs["traits_map"] = config.Attrs["traits_map"]

	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}
