package generic

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/trace"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	tfpath "github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	tftypes "github.com/hashicorp/terraform-plugin-framework/types"
)

type teleportProvider interface {
	GetClient() *client.Client
	IsConfigured(diag.Diagnostics) bool
}

type RetryConfig struct {
	Base     time.Duration
	Cap      time.Duration
	MaxTries int
}

// TODO: can I merge adapter and resource client?

type adapter[T any] interface {
	GetName(T) string
	GetRevision(T) string
	// TODO: Is SetName needed?
	// SetName(T, string)
	// SetRevision(T, string)
}

type resourceClient[T any] interface {
	Get(context.Context, string) (T, error)
	Create(context.Context, T) error
	Upsert(context.Context, T) error
	Delete(context.Context, string) error
}

type tfResource[T any] struct {
	fromTerraform func(context.Context, tftypes.Object, *T) diag.Diagnostics
	toTerraform   func(context.Context, *T, *tftypes.Object) diag.Diagnostics
	kind          string
	tfKind        string
	idPath        tfpath.Path

	a   adapter[*T]
	clt resourceClient[*T]

	retryConfig          RetryConfig
	providerIsConfigured func(diags diag.Diagnostics) bool
}

type tfResourceType[T any] struct {
	schema func(context.Context) (tfsdk.Schema, diag.Diagnostics)
	res    func(provider tfsdk.Provider) (*tfResource[T], error)
}

func (r tfResourceType[T]) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return r.schema(ctx)
}

func (r tfResourceType[T]) NewResource(_ context.Context, p tfsdk.Provider) (tfsdk.Resource, diag.Diagnostics) {
	resource, err := r.res(p)
	if err != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Failed to build resource", "Provider internal resource type building fail. This is a bug.")}
	}
	return resource, nil
}

func (r tfResource[T]) Create(ctx context.Context, req tfsdk.CreateResourceRequest, resp *tfsdk.CreateResourceResponse) {
	if !r.providerIsConfigured(resp.Diagnostics) {
		return
	}

	var plan tftypes.Object
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	res := new(T)
	diags = r.fromTerraform(ctx, plan, res)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: generate name for provision token here

	// TODO: check and set defaults here

	id := r.a.GetName(res)

	_, err := r.clt.Get(ctx, id)
	if !trace.IsNotFound(err) {
		if err == nil {
			resp.Diagnostics.AddError(
				"Resource already exists in Teleport",
				fmt.Sprintf(
					"%s exists in Teleport. Either remove it (tctl rm %s/%s) or import it to the existing state (terraform import %s.%s %s)",
					r.kind, r.kind, id, r.tfKind, id, id,
				))
			return
		}

		resp.Diagnostics.AddError(
			"Error reading resource from Teleport",
			fmt.Sprintf("Failed to read %s/%s from Teleport: %s", r.kind, id, err),
		)
		return
	}

	if err := r.clt.Create(ctx, res); err != nil {
		resp.Diagnostics.AddError(
			"Error creating resource in Teleport",
			fmt.Sprintf("Failed to create %s/%s in Teleport: %s", r.kind, id, err))
		return
	}

	// We try getting the resource until it exists
	// This makes sure the resource was propagated into the cache of the auth we're connetced to.
	var tries int
	retry, err := retryutils.NewRetryV2(
		retryutils.RetryV2Config{
			Driver: retryutils.NewExponentialDriver(r.retryConfig.Base),
			First:  r.retryConfig.Base,
			Max:    r.retryConfig.Cap,
			Jitter: retryutils.HalfJitter,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to initialize retry", fmt.Sprintf("Terraform provider failed to initialize retry: %s", err))
		return
	}

	for {
		tries += 1
		res, err = r.clt.Get(ctx, id)
		if err == nil {
			break
		}
		if !trace.IsNotFound(err) {
			resp.Diagnostics.AddError(
				"Unexpected error when reading the resource back from Teleport",
				fmt.Sprintf("Failed to read %s/%s from Teleport. The resource might still have been created and require a manual `terraform import`: %s", r.kind, id, err))
			return
		}
		if tries >= r.retryConfig.MaxTries {
			resp.Diagnostics.AddError(
				"Resource never appeared in Teleport",
				fmt.Sprintf("Tried %d times to read %s/%s from Teleport. The resource is still missing. The Terraform state might be outdated and you'll need to import the resource.", tries, r.kind, id))
			return
		}
		select {
		case <-ctx.Done():
			resp.Diagnostics.AddError(
				"Context cancelled while Terraform was waiting",
				fmt.Sprintf("The context got cancelled while Terraform was waiting for the resource to appear in Teleport. The resource might still have been created and require a manual `terraform import`: %s", ctx.Err()))
			return
		case <-retry.After():
		}
	}

	resp.Diagnostics.Append(r.toTerraform(ctx, res, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.Attrs["id"] = tftypes.String{Value: id}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r tfResource[T]) Read(ctx context.Context, req tfsdk.ReadResourceRequest, resp *tfsdk.ReadResourceResponse) {
	// TODO: fail if not configured?
	var state tftypes.Object
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var id tftypes.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, r.idPath, &id)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.clt.Get(ctx, id.Value)
	if trace.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading resource in Teleport", fmt.Sprintf("Failed to read %s/%s from Teleport: %s", r.kind, id, err))
		return
	}

	resp.Diagnostics.Append(r.toTerraform(ctx, res, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r tfResource[T]) Update(ctx context.Context, req tfsdk.UpdateResourceRequest, resp *tfsdk.UpdateResourceResponse) {
	if !r.providerIsConfigured(resp.Diagnostics) {
		return
	}

	var plan tftypes.Object
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res := new(T)
	resp.Diagnostics.Append(r.fromTerraform(ctx, plan, res)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: check and set defaults here
	id := r.a.GetName(res)

	resBefore, err := r.clt.Get(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading existing resource in Teleport", fmt.Sprintf("Failed to read %s/%s from Teleport before update: %s", r.kind, id, err))
		return
	}

	if err := r.clt.Upsert(ctx, res); err != nil {
		resp.Diagnostics.AddError("Error updating resource in Teleport", fmt.Sprintf("Failed to update %s/%s in Teleport: %s", r.kind, id, err))
		return
	}

	// We try getting the resource until its revision changes
	// This makes sure the resource was propagated into the cache of the auth we're connected to.
	var tries int
	retry, err := retryutils.NewRetryV2(
		retryutils.RetryV2Config{
			Driver: retryutils.NewExponentialDriver(r.retryConfig.Base),
			First:  r.retryConfig.Base,
			Max:    r.retryConfig.Cap,
			Jitter: retryutils.HalfJitter,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to initialize retry", fmt.Sprintf("Terraform provider failed to initialize retry: %s", err))
		return
	}

	for {
		tries += 1
		res, err = r.clt.Get(ctx, id)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unexpected error when reading the resource back from Teleport",
				fmt.Sprintf("Failed to read %s/%s from Teleport: %s", r.kind, id, err))
			return
		}
		if r.a.GetRevision(res) != r.a.GetRevision(resBefore) {
			// Resource changed, we suppose it's our edit that propagated to the cache.
			break
		}
		if tries >= r.retryConfig.MaxTries {
			resp.Diagnostics.AddError(
				"Resource stale in Teleport",
				fmt.Sprintf("Tried %d times to read %s/%s from Teleport. The resource is still stale. The Terraform state might be outdated and you'll need to import the resource.", tries, r.kind, id))
			return
		}
		select {
		case <-ctx.Done():
			resp.Diagnostics.AddError(
				"Context cancelled while Terraform was waiting",
				fmt.Sprintf("The context got cancelled while Terraform was waiting for the resource to appear in Teleport: %s", ctx.Err()))
			return
		case <-retry.After():
		}
	}

	resp.Diagnostics.Append(r.toTerraform(ctx, res, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r tfResource[T]) Delete(ctx context.Context, req tfsdk.DeleteResourceRequest, resp *tfsdk.DeleteResourceResponse) {
	var id tftypes.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, r.idPath, &id)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// TODO: this is a change in the behaviour (we were not getting before)
	// check if we should keep this
	_, err := r.clt.Get(ctx, id.Value)
	if trace.IsNotFound(err) {
		// Resource isn't here. Nothing to do.
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to read resource in Teleport", fmt.Sprintf("Failed to read %s/%s from Teleport: %s", r.kind, id, err))
		return
	}

	if err := r.clt.Delete(ctx, id.Value); err != nil {
		resp.Diagnostics.AddError("Failed to delete resource in Teleport", fmt.Sprintf("Failed to delete %s/%s from Teleport: %s", r.kind, id, err))
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r tfResource[T]) ImportState(ctx context.Context, req tfsdk.ImportResourceStateRequest, resp *tfsdk.ImportResourceStateResponse) {
	res, err := r.clt.Get(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read resource in Teleport", fmt.Sprintf("Failed to read %s/%s from Teleport: %s", r.kind, req.ID, err))
		return
	}

	var state tftypes.Object
	// TODO: do we need this?
	resp.Diagnostics.Append(resp.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(r.toTerraform(ctx, res, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := r.a.GetName(res)
	state.Attrs["id"] = tftypes.String{Value: id}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
