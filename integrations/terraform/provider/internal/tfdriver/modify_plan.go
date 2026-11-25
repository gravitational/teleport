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

package tfdriver

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ToTerraformPreserveUnknownFunc copies a Teleport resource into Terraform values
// while preserving unknowns.
type ToTerraformPreserveUnknownFunc[T any] func(context.Context, *T, *types.Object, bool) (types.Object, diag.Diagnostics)

// NullToZeroModifyPlan returns a ModifyPlanFunc that normalizes attributes omitted
// from the Terraform config to their Teleport zero value, while preserving
// attributes that are really unknown (e.g. because they depend on another resource
// that hasn't been applied yet).
//
// This allows users to "unset" an Optional+Computed attribute by removing it
// from their config instead of preserving prior state.
//
// Only use this for resources that do not rely on CheckAndSetDefaults to set
// server-side defaults. Using both may result in inconsistent results after
// apply.
func NullToZeroModifyPlan[T any](fromTerraform FromTerraformFunc[T], toTerraform ToTerraformPreserveUnknownFunc[T]) ModifyPlanFunc[T] {
	return func(ctx context.Context, config, plan types.Object) (types.Object, diag.Diagnostics) {
		val := new(T)

		// Skip plan modifier for resources that rely on CheckAndSetDefaults.
		if _, ok := any(val).(interface{ CheckAndSetDefaults() error }); ok {
			return plan, nil
		}

		diags := fromTerraform(ctx, config, val)
		if diags.HasError() {
			return plan, diags
		}

		const preserveUnknown = true
		config, diags = toTerraform(ctx, val, &config, preserveUnknown)
		if diags.HasError() {
			return plan, diags
		}

		attrs := plan.Attributes()
		if spec, exists := config.Attributes()["spec"]; exists {
			attrs["spec"] = spec
		}

		return types.ObjectValue(plan.AttributeTypes(ctx), attrs)
	}
}
