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

package provider

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/utils/retryutils"
)

// dataSourceTeleportType is the resource
type resourceTeleportType[T any, I fmt.Stringer] struct {
	newResourceClient func(*client.Client) resourceClient[T, I]
	kind              string
	schema            func(context.Context) (tfsdk.Schema, diag.Diagnostics)
	toTerraform       func(context.Context, *T, *types.Object) diag.Diagnostics
	fromTerraform     func(context.Context, types.Object, *T) diag.Diagnostics
	resourceRevision  func(*T) string
	propagateFields   func(*T, *T)

	identifierFromState    func(context.Context, tfsdk.State) (I, diag.Diagnostics)
	identifierFromResource func(*T) I
	identifierFromImportID func(string) (I, error)
}

type resourceClient[T any, I fmt.Stringer] interface {
	Get(context.Context, GetResourceRequest[I]) (*T, error)
	Create(context.Context, *T) error
	Upsert(context.Context, *T) error
	Delete(context.Context, I) error
}

// GetSchema returns the resource schema
func (r resourceTeleportType[T, I]) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return r.schema(ctx)
}

// NewResource creates the empty resource
func (r resourceTeleportType[T, I]) NewResource(_ context.Context, p tfsdk.Provider) (tfsdk.Resource, diag.Diagnostics) {
	provider := p.(*Provider)
	return resourceTeleport[T, I]{
		resourceClient: r.newResourceClient(provider.Client),
		resource:       r,
		provider:       provider,
	}, nil
}

// dataSourceTeleport is the resource
type resourceTeleport[T any, I fmt.Stringer] struct {
	resourceClient[T, I]
	resource resourceTeleportType[T, I]
	provider *Provider
}

// Create implements [tfsdk.Resource].
func (r resourceTeleport[T, I]) Create(ctx context.Context, req tfsdk.CreateResourceRequest, resp *tfsdk.CreateResourceResponse) {
	if !r.provider.IsConfigured(resp.Diagnostics) {
		return
	}

	var plan types.Object
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	val := new(T)
	diags = r.resource.fromTerraform(ctx, plan, val)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := r.resource.identifierFromResource(val)
	_, err := r.resourceClient.Get(ctx, GetResourceRequest[I]{Identifier: id, WithSecrets: true})
	if !trace.IsNotFound(err) {
		if err == nil {
			resp.Diagnostics.Append(
				diagFromErr(
					fmt.Sprintf("%q exists in Teleport", r.resource.kind),
					trace.Errorf("%[1]s exists in Teleport. Either remove it (tctl rm %[1]s/%[2]v)"+
						" or import it to the existing state (terraform import teleport_%[1]s.%[2]v %[2]v)", r.resource.kind, id),
				),
			)
			return
		}

		resp.Diagnostics.Append(diagFromWrappedErr(fmt.Sprintf("Error reading %q", r.resource.kind), trace.Wrap(err), r.resource.kind))
		return
	}

	if err := r.resourceClient.Create(ctx, val); err != nil {
		resp.Diagnostics.Append(diagFromWrappedErr(fmt.Sprintf("Error creating %q", r.resource.kind), trace.Wrap(err), r.resource.kind))
		return
	}

	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		Driver: retryutils.NewExponentialDriver(r.provider.RetryConfig.Base),
		First:  r.provider.RetryConfig.Base,
		Max:    r.provider.RetryConfig.Cap,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return
	}

	var retrieved *T
	for tries := 1; ; tries++ {
		retrieved, err = r.resourceClient.Get(ctx, GetResourceRequest[I]{Identifier: id, WithSecrets: true})
		if trace.IsNotFound(err) {
			if tries >= r.provider.RetryConfig.MaxTries {
				diagMessage := fmt.Sprintf("Error reading %q (tried %d times) - state outdated, please import resource", r.resource.kind, tries)
				resp.Diagnostics.AddError(diagMessage, r.resource.kind)
				return
			}

			select {
			case <-ctx.Done():
				resp.Diagnostics.Append(diagFromWrappedErr(fmt.Sprintf("Error reading %q", r.resource.kind), trace.Wrap(ctx.Err()), r.resource.kind))
				return
			case <-retry.After():
			}

			continue
		}
		if err != nil {
			resp.Diagnostics.Append(diagFromWrappedErr(fmt.Sprintf("Error reading %q", r.resource.kind), trace.Wrap(err), r.resource.kind))
			return
		}

		diags = r.resource.toTerraform(ctx, retrieved, &plan)
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

// Read implements [tfsdk.Resource].
func (r resourceTeleport[T, I]) Read(ctx context.Context, req tfsdk.ReadResourceRequest, resp *tfsdk.ReadResourceResponse) {
	var state types.Object
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, diags := r.resource.identifierFromState(ctx, req.State)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	retrieved, err := r.resourceClient.Get(ctx, GetResourceRequest[I]{Identifier: id, WithSecrets: true})
	if trace.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}

	if err != nil {
		resp.Diagnostics.Append(diagFromWrappedErr(fmt.Sprintf("Error reading %q", r.resource.kind), trace.Wrap(err), r.resource.kind))
		return
	}

	diags = r.resource.toTerraform(ctx, retrieved, &state)
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

// Update updates the Teleport resource
func (r resourceTeleport[T, I]) Update(ctx context.Context, req tfsdk.UpdateResourceRequest, resp *tfsdk.UpdateResourceResponse) {
	if !r.provider.IsConfigured(resp.Diagnostics) {
		return
	}

	var plan types.Object
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	val := new(T)
	diags = r.resource.fromTerraform(ctx, plan, val)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := r.resource.identifierFromResource(val)
	resourceBefore, err := r.resourceClient.Get(ctx, GetResourceRequest[I]{Identifier: id, WithSecrets: true})
	if err != nil {
		resp.Diagnostics.Append(diagFromWrappedErr(fmt.Sprintf("Error reading %q", r.resource.kind), err, r.resource.kind))
		return
	}

	if r.resource.propagateFields != nil {
		r.resource.propagateFields(resourceBefore, val)
	}

	if err := r.resourceClient.Upsert(ctx, val); err != nil {
		resp.Diagnostics.Append(diagFromWrappedErr(fmt.Sprintf("Error updating %q", r.resource.kind), err, r.resource.kind))
		return
	}

	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		Driver: retryutils.NewExponentialDriver(r.provider.RetryConfig.Base),
		First:  r.provider.RetryConfig.Base,
		Max:    r.provider.RetryConfig.Cap,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return
	}

	for tries := 1; ; tries++ {
		retrieved, err := r.resourceClient.Get(ctx, GetResourceRequest[I]{Identifier: id, WithSecrets: true})
		if err != nil {
			resp.Diagnostics.Append(diagFromWrappedErr(fmt.Sprintf("Error reading %q", r.resource.kind), err, r.resource.kind))
			return
		}

		if r.resource.resourceRevision(resourceBefore) != r.resource.resourceRevision(retrieved) {
			diags = r.resource.toTerraform(ctx, retrieved, &plan)
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

		if tries >= r.provider.RetryConfig.MaxTries {
			diagMessage := fmt.Sprintf("Error reading %q (tried %d times) - state outdated, please import resource", r.resource.kind, tries)
			resp.Diagnostics.AddError(diagMessage, r.resource.kind)
			return
		}

		select {
		case <-ctx.Done():
			resp.Diagnostics.Append(diagFromWrappedErr(fmt.Sprintf("Error reading %q", r.resource.kind), trace.Wrap(ctx.Err()), r.resource.kind))
			return
		case <-retry.After():
		}
	}
}

// Delete deletes the Teleport resource
func (r resourceTeleport[T, I]) Delete(ctx context.Context, req tfsdk.DeleteResourceRequest, resp *tfsdk.DeleteResourceResponse) {
	id, diags := r.resource.identifierFromState(ctx, req.State)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.resourceClient.Delete(ctx, id); err != nil {
		resp.Diagnostics.Append(diagFromWrappedErr(fmt.Sprintf("Error deleting %q", r.resource.kind), trace.Wrap(err), r.resource.kind))
		return
	}

	resp.State.RemoveResource(ctx)
}

// ImportState imports the Teleport resource state
func (r resourceTeleport[T, I]) ImportState(ctx context.Context, req tfsdk.ImportResourceStateRequest, resp *tfsdk.ImportResourceStateResponse) {
	id, err := r.resource.identifierFromImportID(req.ID)
	if err != nil {
		resp.Diagnostics.Append(diagFromWrappedErr(fmt.Sprintf("Invalid identifier for %q", r.resource.kind), trace.Wrap(err), r.resource.kind))
		return
	}

	retrieved, err := r.resourceClient.Get(ctx, GetResourceRequest[I]{Identifier: id, WithSecrets: true})
	if err != nil {
		resp.Diagnostics.Append(diagFromWrappedErr(fmt.Sprintf("Error reading %q", r.resource.kind), trace.Wrap(err), r.resource.kind))
		return
	}

	var state types.Object
	diags := resp.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = r.resource.toTerraform(ctx, retrieved, &state)
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
