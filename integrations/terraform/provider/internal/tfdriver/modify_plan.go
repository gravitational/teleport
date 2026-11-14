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
