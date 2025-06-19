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
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timetypes/timetypes"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewKubernetesDataSource() datasource.DataSource {
	return &KubernetesDataSource{}
}

type KubernetesDataSourceModelSelector struct {
	Name types.String `tfsdk:"name"`
}

type KubernetesDataSourceModel struct {
	// Arguments
	Selector      KubernetesDataSourceModelSelector `tfsdk:"selector"`
	CredentialTTL timetypes.GoDuration              `tfsdk:"credential_ttl"`

	// Attributes
	Output *KubernetesDataSourceModelOutput `tfsdk:"output"`
}

type KubernetesDataSourceModelOutput struct {
	ClientKey            types.String `tfsdk:"client_key"`
	Host                 types.String `tfsdk:"host"`
	TLSServerName        types.String `tfsdk:"tls_server_name"`
	ClientCertificate    types.String `tfsdk:"client_certificate"`
	ClusterCACertificate types.String `tfsdk:"cluster_ca_certificate"`
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
		MarkdownDescription: "The Kubernetes data source provides credentials to allow other providers to access Kubernetes cluster through Teleport Machine & Workload Identity.",

		Attributes: map[string]schema.Attribute{
			// Arguments
			"selector": schema.SingleNestedAttribute{
				MarkdownDescription: "Selects the Kubernetes cluster to connect to.",
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						MarkdownDescription: "The name of the Kubernetes cluster to connect to.",
						Required:            true,
					},
				},
				Required: true,
			},
			"credential_ttl": schema.StringAttribute{
				CustomType:          timetypes.GoDurationType{},
				MarkdownDescription: "How long the issued credentials should be valid for. Defaults to 30 minutes.",
				Optional:            true,
				Computed:            true,
			},
			// Attributes
			"output": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"client_key": schema.StringAttribute{
						Computed:            true,
						Sensitive:           true,
						MarkdownDescription: "Compatible with the `client_key` argument of the `kubernetes` provider.",
					},
					"host": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Compatible with the `host` argument of the `kubernetes` provider.",
					},
					"tls_server_name": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Compatible with the `tls_server_name` argument of the `kubernetes` provider.",
					},
					"client_certificate": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Compatible with the `client_certificate` argument of the `kubernetes` provider.",
					},
					"cluster_ca_certificate": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Compatible with the `cluster_ca_certificate` argument of the `kubernetes` provider.",
					},
				},
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

func (r *KubernetesDataSource) loadModelAndSetDefaults(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) KubernetesDataSourceModel {
	var data KubernetesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return data
	}

	// Set default for credential TTL if not provided.
	if data.CredentialTTL.IsNull() || data.CredentialTTL.IsUnknown() {
		data.CredentialTTL = timetypes.NewGoDurationValue(
			time.Minute * 30,
		)
	}

	return data
}

func (d *KubernetesDataSource) Read(
	ctx context.Context,
	req datasource.ReadRequest,
	resp *datasource.ReadResponse,
) {
	data := d.loadModelAndSetDefaults(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		return
	}

	out := KubernetesDataSourceModelOutput{
		Host: types.StringValue(
			fmt.Sprintf("Hello, %s!", data.Selector.Name.ValueString()),
		),
	}

	data.Output = &out
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
