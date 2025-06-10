/*
Copyright 2015-2025 Gravitational, Inc.

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
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ ephemeral.EphemeralResourceWithConfigure = &KubernetesEphemeralResource{}

func NewKubernetesEphemeralResource() ephemeral.EphemeralResource {
	return &KubernetesEphemeralResource{}
}

type KubernetesEphemeralResource struct{}

func (r *KubernetesEphemeralResource) Metadata(
	_ context.Context,
	req ephemeral.MetadataRequest,
	resp *ephemeral.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes"
}

type KubernetesEphemeralResourceModel struct {
	// Arguments
	ExampleInput types.String `tfsdk:"example_input"`

	// Attributes
	ExampleOutput types.String `tfsdk:"example_output"`
}

func (r *KubernetesEphemeralResource) Schema(
	_ context.Context,
	_ ephemeral.SchemaRequest,
	resp *ephemeral.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "TODO",

		Attributes: map[string]schema.Attribute{
			// Input
			"example_input": schema.StringAttribute{
				MarkdownDescription: "TODO",
				Required:            true,
			},
			// Output
			"example_output": schema.StringAttribute{
				MarkdownDescription: "TODO",
				Computed:            true,
			},
		},
	}
}

func (d *KubernetesEphemeralResource) Configure(
	ctx context.Context,
	req ephemeral.ConfigureRequest,
	resp *ephemeral.ConfigureResponse,
) {
	// TODO: Fetch bot config data from the provider.
}

func (r *KubernetesEphemeralResource) Open(
	ctx context.Context,
	req ephemeral.OpenRequest,
	resp *ephemeral.OpenResponse,
) {
	var data KubernetesEphemeralResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ExampleOutput = types.StringValue(
		fmt.Sprintf("Hello, %s!", data.ExampleInput.ValueString()),
	)
	resp.Diagnostics.Append(resp.Result.Set(ctx, &data)...)
}
