/*
Copyright 2025 Gravitational, Inc.

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

package tfschema

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	tftypes "github.com/hashicorp/terraform-plugin-framework/types"

	apitypes "github.com/gravitational/teleport/api/types"
)

const (
	DefaultRoleOptionsModifierErrSummary         = "DefaultRoleOptions modifier failed"
	DefaultKubernetesResourcesModifierErrSummary = "DefaultKubernetesResources modifier failed"

	DefaultModiferDescription = `The state contains server-generated defaults (in fact they are generated in the pre-apply plan).
However, those defaults become outdated if the version or the default logic changes.
One way to deal with version change is to force-recreate, but this is too destructive.
The workaround we found was to use this plan modifier.`
)

// DefaultRoleOptions returns the default implementation of the DefaultRoleOptionsModifier
func DefaultRoleOptions() tfsdk.AttributePlanModifier {
	return DefaultRoleOptionsModifier{}
}

// DefaultRoleOptionsModifier implements the tfsdk.AttributePlanModifier interface. It accounts
// for default values applied by CheckAndSetDefaults that would otherwise create inconsistent states
type DefaultRoleOptionsModifier struct {
}

// Description of the RoleOptions plan modifier
func (d DefaultRoleOptionsModifier) Description(ctx context.Context) string {
	return "This modifier re-renders the role.spec.options from the user provided config instead of using the state. " +
		DefaultModiferDescription
}

// MarkdownDescription of the RoleOptions plan modifier
func (d DefaultRoleOptionsModifier) MarkdownDescription(ctx context.Context) string {
	return "This modifier re-renders the role.spec.options from the user provided config instead of using the state. " +
		DefaultModiferDescription
}

// Modify the terraform plan to account for defaults applied to RoleOptions by CheckAndSetDefaults
func (d DefaultRoleOptionsModifier) Modify(ctx context.Context, req tfsdk.ModifyAttributePlanRequest, resp *tfsdk.ModifyAttributePlanResponse) {
	var config tftypes.Object
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError(DefaultRoleOptionsModifierErrSummary, "Failed to get config.")
		return
	}

	role := &apitypes.RoleV6{}
	diags = CopyRoleV6FromTerraform(ctx, config, role)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError(DefaultRoleOptionsModifierErrSummary, "Failed to create a role from the config.")
		return
	}

	err := role.CheckAndSetDefaults()
	if err != nil {
		resp.Diagnostics.AddError(DefaultRoleOptionsModifierErrSummary, fmt.Sprintf("Failed to set the role defaults: %s", err))
		return
	}

	diags = CopyRoleV6ToTerraform(ctx, role, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError(DefaultRoleOptionsModifierErrSummary, "Failed to convert back the role into a TF object.")
		return
	}

	specRaw, ok := config.Attrs["spec"]
	if !ok {
		resp.Diagnostics.AddError(DefaultRoleOptionsModifierErrSummary, "Failed to get 'spec' from TF object.")
		return
	}
	spec, ok := specRaw.(tftypes.Object)
	if !ok {
		resp.Diagnostics.AddError(DefaultRoleOptionsModifierErrSummary, "Failed to cast 'spec' as a TF object.")
		return
	}
	optionsRaw, ok := spec.Attrs["options"]
	if !ok {
		resp.Diagnostics.AddError(DefaultRoleOptionsModifierErrSummary, "Failed to get 'options' from TF object.")
		return
	}
	options, ok := optionsRaw.(tftypes.Object)
	if !ok {
		resp.Diagnostics.AddError(DefaultRoleOptionsModifierErrSummary, "Failed to cast 'options' as a TF object.")
		return
	}
	options.Null = false
	resp.AttributePlan = options
}

// DefaultKubernetesResources returns the default implementation of the DefaultKubernetesResourcesModifier
func DefaultKubernetesResources() tfsdk.AttributePlanModifier {
	return DefaultKubernetesResourcesModifier{}
}

// DefaultKubernetesResourcesModifier implements the tfsdk.AttributePlanModifier interface. It accounts
// for default values applied by CheckAndSetDefaults that would otherwise create inconsistent states
type DefaultKubernetesResourcesModifier struct{}

// Description of the KubernetesResources plan modifier
func (d DefaultKubernetesResourcesModifier) Description(_ context.Context) string {
	return "This modifier re-renders the role.spec.allow.kubernetes_resources from the user provided config instead of using the state. " +
		DefaultModiferDescription
}

// MarkdownDescription of the KubernetesResources plan modifier
func (d DefaultKubernetesResourcesModifier) MarkdownDescription(_ context.Context) string {
	return "This modifier re-renders the role.spec.allow.kubernetes_resources from the user provided config instead of using the state. " +
		DefaultModiferDescription
}

// Modify the terraform plan to account for defaults applied to KubernetesResources by CheckAndSetDefaults
func (d DefaultKubernetesResourcesModifier) Modify(ctx context.Context, req tfsdk.ModifyAttributePlanRequest, resp *tfsdk.ModifyAttributePlanResponse) {
	var config tftypes.Object
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError(DefaultKubernetesResourcesModifierErrSummary, "Failed to get config.")
		return
	}

	role := &apitypes.RoleV6{}
	diags = CopyRoleV6FromTerraform(ctx, config, role)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError(DefaultKubernetesResourcesModifierErrSummary, "Failed to create a role from the config.")
		return
	}

	err := role.CheckAndSetDefaults()
	if err != nil {
		resp.Diagnostics.AddError(DefaultKubernetesResourcesModifierErrSummary, fmt.Sprintf("Failed to set the role defaults: %s", err))
		return
	}

	diags = CopyRoleV6ToTerraform(ctx, role, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError(DefaultKubernetesResourcesModifierErrSummary, "Failed to convert back the role into a TF object.")
		return
	}

	specRaw, ok := config.Attrs["spec"]
	if !ok {
		resp.Diagnostics.AddError(DefaultKubernetesResourcesModifierErrSummary, "Failed to get 'spec' from TF object.")
		return
	}
	spec, ok := specRaw.(tftypes.Object)
	if !ok {
		resp.Diagnostics.AddError(DefaultKubernetesResourcesModifierErrSummary, "Failed to cast 'spec' as a TF object.")
		return
	}

	allowRaw, ok := spec.Attrs["allow"]
	if !ok {
		resp.Diagnostics.AddError(DefaultKubernetesResourcesModifierErrSummary, "Failed to cast 'allow' as a TF object.")
		return
	}

	allow, ok := allowRaw.(tftypes.Object)
	if !ok {
		resp.Diagnostics.AddError(DefaultKubernetesResourcesModifierErrSummary, "Failed to cast 'allow' as a TF object.")
		return
	}

	kubernetesResourcesRaw, ok := allow.Attrs["kubernetes_resources"]
	if !ok {
		resp.Diagnostics.AddError(DefaultKubernetesResourcesModifierErrSummary, "Failed to cast 'kubernetes_resources' as a TF list.")
		return
	}

	kubernetesResources, ok := kubernetesResourcesRaw.(tftypes.List)
	if !ok {
		resp.Diagnostics.AddError(DefaultKubernetesResourcesModifierErrSummary, "Failed to cast 'kubernetes_resources' as a TF list.")
		return
	}
	kubernetesResources.Null = false
	resp.AttributePlan = kubernetesResources
}
