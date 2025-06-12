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

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewKubernetesDataSource() datasource.DataSource {
	return &KubernetesDataSource{}
}

type KubernetesDataSourceModel struct {
	// Arguments
	ExampleInput types.String `tfsdk:"example_input"`

	// Attributes
	ExampleOutput types.String `tfsdk:"example_output"`
}

type KubernetesDataSource struct{}

func (d *KubernetesDataSource) Metadata(
	ctx context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes"
}

func (d *KubernetesDataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
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

func (d *KubernetesDataSource) Configure(
	ctx context.Context,
	req datasource.ConfigureRequest,
	resp *datasource.ConfigureResponse,
) {
	// TODO: Fetch bot config data from the provider.
}

func (d *KubernetesDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	var data KubernetesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.ExampleOutput = types.StringValue(
		fmt.Sprintf("Hello, %s!", data.ExampleInput.ValueString()),
	)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
