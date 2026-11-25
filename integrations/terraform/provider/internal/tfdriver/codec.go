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

	datasource_schema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	resource_schema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ResourceCodec converts between Terraform values and a Teleport resource.
type ResourceCodec[T any] interface {
	// Schema returns the Terraform schema.
	Schema(context.Context) (resource_schema.Schema, diag.Diagnostics)
	// FromPlan copies Terraform plan values into a Teleport resource.
	FromPlan(context.Context, types.Object, *T) diag.Diagnostics
	// FromState copies Terraform state values into a Teleport resource.
	FromState(context.Context, types.Object, *T) diag.Diagnostics
	// ToState copies a Teleport resource into Terraform state values.
	ToState(context.Context, *T, *types.Object) (types.Object, diag.Diagnostics)
	// ModifyPlan modifies the Terraform plan.
	ModifyPlan(context.Context, types.Object, types.Object) (types.Object, diag.Diagnostics)
}

// DataSourceCodec converts Terraform data source values to and from Teleport resources.
type DataSourceCodec[T any] interface {
	// Schema returns the Terraform schema.
	Schema(context.Context) (datasource_schema.Schema, diag.Diagnostics)
	// FromConfig copies Terraform config values into a Teleport resource.
	FromConfig(context.Context, types.Object, *T) diag.Diagnostics
	// ToState copies a Teleport resource into Terraform state values.
	ToState(context.Context, *T, *types.Object) (types.Object, diag.Diagnostics)
}

// SchemaFunc returns a Terraform schema.
type SchemaFunc[T any] func(context.Context) (T, diag.Diagnostics)

// FromTerraformFunc copies Terraform values into a Teleport resource.
type FromTerraformFunc[T any] func(context.Context, types.Object, *T) diag.Diagnostics

// ToTerraformFunc copies a Teleport resource into Terraform values.
type ToTerraformFunc[T any] func(context.Context, *T, *types.Object) (types.Object, diag.Diagnostics)

// ModifyPlanFunc modifies the Terraform plan.
type ModifyPlanFunc[T any] func(ctx context.Context, config, plan types.Object) (types.Object, diag.Diagnostics)

// ResourceCodecFuncs adapts functions to ResourceCodec.
type ResourceCodecFuncs[T any] struct {
	SchemaFunc     SchemaFunc[resource_schema.Schema]
	FromPlanFunc   FromTerraformFunc[T]
	FromStateFunc  FromTerraformFunc[T]
	ToStateFunc    ToTerraformFunc[T]
	ModifyPlanFunc ModifyPlanFunc[T]
}

// Schema returns the Terraform schema.
func (c ResourceCodecFuncs[T]) Schema(ctx context.Context) (resource_schema.Schema, diag.Diagnostics) {
	return c.SchemaFunc(ctx)
}

// FromPlan copies Terraform plan values into a Teleport resource.
func (c ResourceCodecFuncs[T]) FromPlan(ctx context.Context, object types.Object, resource *T) diag.Diagnostics {
	return c.FromPlanFunc(ctx, object, resource)
}

// FromState copies Terraform state values into a Teleport resource.
func (c ResourceCodecFuncs[T]) FromState(ctx context.Context, object types.Object, resource *T) diag.Diagnostics {
	return c.FromStateFunc(ctx, object, resource)
}

// ToState copies a Teleport resource into Terraform state values.
func (c ResourceCodecFuncs[T]) ToState(ctx context.Context, resource *T, object *types.Object) (types.Object, diag.Diagnostics) {
	return c.ToStateFunc(ctx, resource, object)
}

// ModifyPlan modifies the Terraform plan.
func (c ResourceCodecFuncs[T]) ModifyPlan(ctx context.Context, config, plan types.Object) (types.Object, diag.Diagnostics) {
	if c.ModifyPlanFunc == nil {
		return plan, nil
	}
	return c.ModifyPlanFunc(ctx, config, plan)
}

// DataSourceCodecFuncs adapts functions to DataSourceCodec.
type DataSourceCodecFuncs[T any] struct {
	SchemaFunc     SchemaFunc[datasource_schema.Schema]
	FromConfigFunc FromTerraformFunc[T]
	ToStateFunc    ToTerraformFunc[T]
}

// Schema returns the Terraform schema.
func (c DataSourceCodecFuncs[T]) Schema(ctx context.Context) (datasource_schema.Schema, diag.Diagnostics) {
	return c.SchemaFunc(ctx)
}

// FromConfig copies Terraform config values into a Teleport resource.
func (c DataSourceCodecFuncs[T]) FromConfig(ctx context.Context, object types.Object, resource *T) diag.Diagnostics {
	if c.FromConfigFunc == nil {
		return nil
	}
	return c.FromConfigFunc(ctx, object, resource)
}

// ToState copies a Teleport resource into Terraform state values.
func (c DataSourceCodecFuncs[T]) ToState(ctx context.Context, resource *T, object *types.Object) (types.Object, diag.Diagnostics) {
	return c.ToStateFunc(ctx, resource, object)
}
