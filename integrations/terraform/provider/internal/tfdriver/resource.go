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

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/gravitational/teleport/api/utils/retryutils"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdiag"
)

// Runtime provides provider settings for Terraform resources.
type Runtime interface {
	// IsConfigured reports whether the provider is ready to use.
	IsConfigured(diag.Diagnostics) bool
	// Retry returns the retry strategy for resource reads.
	Retry() (retryutils.Retry, error)
	// MaxRetries returns the maximum number of retry attempts.
	MaxRetries() int
}

// ResourceClient performs operations for a Terraform resource.
type ResourceClient[T any, I Identifier] interface {
	DataSourceClient[T, I]
	// Create creates a Teleport resource.
	Create(context.Context, *T) error
	// Upsert updates a Teleport resource.
	Upsert(context.Context, *T) error
	// Delete deletes a Teleport resource by identifier.
	Delete(context.Context, I) error
}

// UpdatePreparer prepares a resource before update.
type UpdatePreparer[T any] interface {
	// PrepareUpdate prepares a resource before update.
	PrepareUpdate(*T, *T) error
}

// ResourceType describes a Terraform resource.
type ResourceType[T any, I Identifier] struct {
	NewResourceClient func(tfsdk.Provider) ResourceClient[T, I]
	Kind              string
	Codec             ResourceCodec[T]
	Normalizer        ResourceNormalizer[T]
	ResourceRevision  func(*T) string
	Identifier        IdentifierPolicy[T, I]
}

// GetSchema returns the resource schema.
func (r ResourceType[T, I]) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return r.Codec.Schema(ctx)
}

// NewResource creates the resource.
func (r ResourceType[T, I]) NewResource(_ context.Context, p tfsdk.Provider) (tfsdk.Resource, diag.Diagnostics) {
	return Resource[T, I]{
		resourceClient: r.NewResourceClient(p),
		resource:       r,
		runtime:        p.(Runtime),
	}, nil
}

// Resource handles Terraform actions for one Teleport resource type.
type Resource[T any, I Identifier] struct {
	resourceClient ResourceClient[T, I]
	resource       ResourceType[T, I]
	runtime        Runtime
}

func (r Resource[T, I]) normalizeCreate(ctx context.Context, resource *T) error {
	if r.resource.Normalizer == nil {
		return nil
	}

	return r.resource.Normalizer.NormalizeCreate(ctx, resource)
}

func (r Resource[T, I]) normalizeUpdate(ctx context.Context, resource *T) error {
	if r.resource.Normalizer == nil {
		return nil
	}

	return r.resource.Normalizer.NormalizeUpdate(ctx, resource)
}

// Create creates the Teleport resource.
func (r Resource[T, I]) Create(ctx context.Context, req tfsdk.CreateResourceRequest, resp *tfsdk.CreateResourceResponse) {
	if !r.runtime.IsConfigured(resp.Diagnostics) {
		return
	}

	var plan types.Object
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	val := new(T)
	diags = r.resource.Codec.FromPlan(ctx, plan, val)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.normalizeCreate(ctx, val); err != nil {
		resp.Diagnostics.Append(tfdiag.DiagFromWrappedErr(fmt.Sprintf("Error preparing %q", r.resource.Kind), err, r.resource.Kind))
		return
	}

	id := r.resource.Identifier.FromResource(val)
	_, err := r.resourceClient.Get(ctx, id)
	if !trace.IsNotFound(err) {
		if err == nil {
			resp.Diagnostics.Append(
				tfdiag.DiagFromErr(
					fmt.Sprintf("%q exists in Teleport", r.resource.Kind),
					trace.Errorf("%[1]s exists in Teleport. Either remove it (tctl rm %[1]s/%[2]v)"+
						" or import it to the existing state (terraform import teleport_%[1]s.%[2]v %[2]v)", r.resource.Kind, id),
				),
			)
			return
		}

		resp.Diagnostics.Append(tfdiag.DiagFromWrappedErr(fmt.Sprintf("Error reading %q", r.resource.Kind), trace.Wrap(err), r.resource.Kind))
		return
	}

	if err := r.resourceClient.Create(ctx, val); err != nil {
		resp.Diagnostics.Append(tfdiag.DiagFromWrappedErr(fmt.Sprintf("Error creating %q", r.resource.Kind), trace.Wrap(err), r.resource.Kind))
		return
	}

	retry, err := r.runtime.Retry()
	if err != nil {
		return
	}

	for tries := 1; ; tries++ {
		retrieved, err := r.resourceClient.Get(ctx, id)
		if trace.IsNotFound(err) {
			if tries >= r.runtime.MaxRetries() {
				diagMessage := fmt.Sprintf("Error reading %q (tried %d times) - state outdated, please import resource", r.resource.Kind, tries)
				resp.Diagnostics.AddError(diagMessage, r.resource.Kind)
				return
			}

			select {
			case <-ctx.Done():
				resp.Diagnostics.Append(tfdiag.DiagFromWrappedErr(fmt.Sprintf("Error reading %q", r.resource.Kind), trace.Wrap(ctx.Err()), r.resource.Kind))
				return
			case <-retry.After():
			}

			continue
		}
		if err != nil {
			resp.Diagnostics.Append(tfdiag.DiagFromWrappedErr(fmt.Sprintf("Error reading %q", r.resource.Kind), trace.Wrap(err), r.resource.Kind))
			return
		}

		diags = r.resource.Codec.ToState(ctx, retrieved, &plan)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		plan.Attrs["id"] = types.String{Value: id.String()}

		diags = resp.State.Set(ctx, &plan)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		return
	}
}

// Read reads the Teleport resource.
func (r Resource[T, I]) Read(ctx context.Context, req tfsdk.ReadResourceRequest, resp *tfsdk.ReadResourceResponse) {
	var state types.Object
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, diags := r.resource.Identifier.FromState(ctx, req.State)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	retrieved, err := r.resourceClient.Get(ctx, id)
	if trace.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.Append(tfdiag.DiagFromWrappedErr(fmt.Sprintf("Error reading %q", r.resource.Kind), trace.Wrap(err), r.resource.Kind))
		return
	}

	diags = r.resource.Codec.ToState(ctx, retrieved, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the Teleport resource.
func (r Resource[T, I]) Update(ctx context.Context, req tfsdk.UpdateResourceRequest, resp *tfsdk.UpdateResourceResponse) {
	if !r.runtime.IsConfigured(resp.Diagnostics) {
		return
	}

	var plan types.Object
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	val := new(T)
	diags = r.resource.Codec.FromPlan(ctx, plan, val)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.normalizeUpdate(ctx, val); err != nil {
		resp.Diagnostics.Append(tfdiag.DiagFromWrappedErr(fmt.Sprintf("Error preparing %q update", r.resource.Kind), err, r.resource.Kind))
		return
	}

	id := r.resource.Identifier.FromResource(val)
	resourceBefore, err := r.resourceClient.Get(ctx, id)
	if err != nil {
		resp.Diagnostics.Append(tfdiag.DiagFromWrappedErr(fmt.Sprintf("Error reading %q", r.resource.Kind), err, r.resource.Kind))
		return
	}

	if updatePreparer, ok := r.resourceClient.(UpdatePreparer[T]); ok {
		if err := updatePreparer.PrepareUpdate(resourceBefore, val); err != nil {
			resp.Diagnostics.Append(tfdiag.DiagFromWrappedErr(fmt.Sprintf("Error preparing %q update", r.resource.Kind), err, r.resource.Kind))
			return
		}
	}

	if err := r.resourceClient.Upsert(ctx, val); err != nil {
		resp.Diagnostics.Append(tfdiag.DiagFromWrappedErr(fmt.Sprintf("Error updating %q", r.resource.Kind), err, r.resource.Kind))
		return
	}

	retry, err := r.runtime.Retry()
	if err != nil {
		return
	}

	for tries := 1; ; tries++ {
		retrieved, err := r.resourceClient.Get(ctx, id)
		if err != nil {
			resp.Diagnostics.Append(tfdiag.DiagFromWrappedErr(fmt.Sprintf("Error reading %q", r.resource.Kind), err, r.resource.Kind))
			return
		}

		if r.resource.ResourceRevision(resourceBefore) != r.resource.ResourceRevision(retrieved) {
			diags = r.resource.Codec.ToState(ctx, retrieved, &plan)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}

			diags = resp.State.Set(ctx, plan)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}

			return
		}

		if tries >= r.runtime.MaxRetries() {
			diagMessage := fmt.Sprintf("Error reading %q (tried %d times) - state outdated, please import resource", r.resource.Kind, tries)
			resp.Diagnostics.AddError(diagMessage, r.resource.Kind)
			return
		}

		select {
		case <-ctx.Done():
			resp.Diagnostics.Append(tfdiag.DiagFromWrappedErr(fmt.Sprintf("Error reading %q", r.resource.Kind), trace.Wrap(ctx.Err()), r.resource.Kind))
			return
		case <-retry.After():
		}
	}
}

// Delete deletes the Teleport resource.
func (r Resource[T, I]) Delete(ctx context.Context, req tfsdk.DeleteResourceRequest, resp *tfsdk.DeleteResourceResponse) {
	id, diags := r.resource.Identifier.FromState(ctx, req.State)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.resourceClient.Delete(ctx, id); err != nil {
		resp.Diagnostics.Append(tfdiag.DiagFromWrappedErr(fmt.Sprintf("Error deleting %q", r.resource.Kind), trace.Wrap(err), r.resource.Kind))
		return
	}

	resp.State.RemoveResource(ctx)
}

// ImportState imports the Teleport resource state.
func (r Resource[T, I]) ImportState(ctx context.Context, req tfsdk.ImportResourceStateRequest, resp *tfsdk.ImportResourceStateResponse) {
	id, err := r.resource.Identifier.FromImportID(req.ID)
	if err != nil {
		resp.Diagnostics.Append(tfdiag.DiagFromWrappedErr(fmt.Sprintf("Invalid identifier for %q", r.resource.Kind), trace.Wrap(err), r.resource.Kind))
		return
	}

	retrieved, err := r.resourceClient.Get(ctx, id)
	if err != nil {
		resp.Diagnostics.Append(tfdiag.DiagFromWrappedErr(fmt.Sprintf("Error reading %q", r.resource.Kind), trace.Wrap(err), r.resource.Kind))
		return
	}

	var state types.Object
	diags := resp.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = r.resource.Codec.ToState(ctx, retrieved, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Attrs["id"] = types.String{Value: req.ID}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
