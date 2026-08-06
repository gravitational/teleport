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
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ResourceCodec converts between Terraform values and a Teleport resource.
type ResourceCodec[T any] interface {
	// Schema returns the Terraform schema.
	Schema(context.Context) (tfsdk.Schema, diag.Diagnostics)
	// FromConfig copies Terraform config values into a Teleport resource.
	FromConfig(context.Context, types.Object, *T) diag.Diagnostics
	// FromPlan copies Terraform plan values into a Teleport resource.
	FromPlan(context.Context, types.Object, *T) diag.Diagnostics
	// FromState copies Terraform state values into a Teleport resource.
	FromState(context.Context, types.Object, *T) diag.Diagnostics
	// ToConfig copies a resource back into Terraform config values.
	ToConfig(context.Context, *T, *types.Object) diag.Diagnostics
	// ToState copies a Teleport resource into Terraform state values.
	ToState(context.Context, *T, *types.Object) diag.Diagnostics
}

// DataSourceCodec converts Terraform data source values to and from Teleport resources.
type DataSourceCodec[T any] interface {
	// Schema returns the Terraform schema.
	Schema(context.Context) (tfsdk.Schema, diag.Diagnostics)
	// FromConfig copies Terraform config values into a Teleport resource.
	FromConfig(context.Context, types.Object, *T) diag.Diagnostics
	// ToState copies a Teleport resource into Terraform state values.
	ToState(context.Context, *T, *types.Object) diag.Diagnostics
}

// SchemaFunc returns a Terraform schema.
type SchemaFunc func(context.Context) (tfsdk.Schema, diag.Diagnostics)

// FromTerraformFunc copies Terraform values into a Teleport resource.
type FromTerraformFunc[T any] func(context.Context, types.Object, *T) diag.Diagnostics

// ToTerraformFunc copies a Teleport resource into Terraform values.
type ToTerraformFunc[T any] func(context.Context, *T, *types.Object) diag.Diagnostics

// ResourceCodecFuncs adapts functions to ResourceCodec.
type ResourceCodecFuncs[T any] struct {
	SchemaFunc     SchemaFunc
	FromConfigFunc FromTerraformFunc[T]
	FromPlanFunc   FromTerraformFunc[T]
	FromStateFunc  FromTerraformFunc[T]
	ToConfigFunc   ToTerraformFunc[T]
	ToStateFunc    ToTerraformFunc[T]
}

// Schema returns the Terraform schema.
func (c ResourceCodecFuncs[T]) Schema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	if c.SchemaFunc == nil {
		return tfsdk.Schema{}, missingCodecFuncDiagnostics("SchemaFunc")
	}
	return c.SchemaFunc(ctx)
}

// FromConfig copies Terraform config values into a Teleport resource.
func (c ResourceCodecFuncs[T]) FromConfig(ctx context.Context, object types.Object, resource *T) diag.Diagnostics {
	if c.FromConfigFunc == nil {
		return missingCodecFuncDiagnostics("FromConfigFunc")
	}
	return c.FromConfigFunc(ctx, object, resource)
}

// FromPlan copies Terraform plan values into a Teleport resource.
func (c ResourceCodecFuncs[T]) FromPlan(ctx context.Context, object types.Object, resource *T) diag.Diagnostics {
	if c.FromPlanFunc == nil {
		return missingCodecFuncDiagnostics("FromPlanFunc")
	}
	return c.FromPlanFunc(ctx, object, resource)
}

// FromState copies Terraform state values into a Teleport resource.
func (c ResourceCodecFuncs[T]) FromState(ctx context.Context, object types.Object, resource *T) diag.Diagnostics {
	if c.FromStateFunc == nil {
		return missingCodecFuncDiagnostics("FromStateFunc")
	}
	return c.FromStateFunc(ctx, object, resource)
}

// ToConfig copies a Teleport resource into Terraform values.
func (c ResourceCodecFuncs[T]) ToConfig(ctx context.Context, resource *T, object *types.Object) diag.Diagnostics {
	if c.ToConfigFunc == nil {
		return missingCodecFuncDiagnostics("ToConfigFunc")
	}
	return c.ToConfigFunc(ctx, resource, object)
}

// ToState copies a Teleport resource into Terraform state values.
func (c ResourceCodecFuncs[T]) ToState(ctx context.Context, resource *T, object *types.Object) diag.Diagnostics {
	if c.ToStateFunc == nil {
		return missingCodecFuncDiagnostics("ToStateFunc")
	}
	return c.ToStateFunc(ctx, resource, object)
}

// DataSourceCodecFuncs adapts functions to DataSourceCodec.
type DataSourceCodecFuncs[T any] struct {
	SchemaFunc     SchemaFunc
	FromConfigFunc FromTerraformFunc[T]
	ToStateFunc    ToTerraformFunc[T]
}

// Schema returns the Terraform schema.
func (c DataSourceCodecFuncs[T]) Schema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	if c.SchemaFunc == nil {
		return tfsdk.Schema{}, missingCodecFuncDiagnostics("SchemaFunc")
	}
	return c.SchemaFunc(ctx)
}

// FromConfig copies Terraform config values into a Teleport resource.
func (c DataSourceCodecFuncs[T]) FromConfig(ctx context.Context, object types.Object, resource *T) diag.Diagnostics {
	if c.FromConfigFunc == nil {
		return missingCodecFuncDiagnostics("FromConfigFunc")
	}
	return c.FromConfigFunc(ctx, object, resource)
}

// ToState copies a Teleport resource into Terraform state values.
func (c DataSourceCodecFuncs[T]) ToState(ctx context.Context, resource *T, object *types.Object) diag.Diagnostics {
	if c.ToStateFunc == nil {
		return missingCodecFuncDiagnostics("ToStateFunc")
	}
	return c.ToStateFunc(ctx, resource, object)
}

func missingCodecFuncDiagnostics(function string) diag.Diagnostics {
	return diag.Diagnostics{
		diag.NewErrorDiagnostic(
			"Missing Terraform codec function",
			fmt.Sprintf("%s must be specified for this Terraform codec.", function),
		),
	}
}
